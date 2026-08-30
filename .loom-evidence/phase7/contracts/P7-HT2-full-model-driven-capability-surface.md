# P7-HT2 - Full model-driven Loom capability surface

Status: `ACCEPTED / COMPLETE / INSTALLED BUILD 324 / CLAUDE LIVE N/A BY USER`

Date: 2026-08-31

## Goal

Make the selected Conversation model capable of understanding ordinary user
language and choosing narrow Loom Harness tools for the complete product
surface. Tools may read bounded non-secret metadata or prepare a governed
Proposal. They never receive confirmation, credential, policy, Journal,
StateWriter or terminal Run/Attempt authority.

This contract extends P7-HT1 without replacing its Conversation alignment,
Segment, Context Capsule or confirmation rules.

## Capability families

1. Conversation and Session: search, align, route, model, reasoning, stop and
   workspace review.
2. Mission: search, create draft, inspect progress and prepare blocked-work
   guidance.
3. Agent Team: search, create draft, inspect role bindings and prepare edits.
4. RoundTable: inspect the Mission-linked discussion, open setup and prepare
   bounded interventions.
5. Governance and setup: status, Needs You, diagnostics, Runtime and Provider
   Account health. Credentials are never model-visible or model-writable.

Every mutation-capable tool has a closed input schema and returns a typed,
digest-bound Proposal. There is no generic execute route and no model-facing
confirm, cancel or run tool.

## Work items

### P7-HT2-A - Shared proposal transport

`CURRENT / SOURCE GREEN`

- A control turn can carry both Session Alignment Proposals and Conversation
  Action Proposals without weakening either schema.
- Proposal identity, Tool ID, action, argument, target content, Segment,
  Attempt, expiry and digest are frozen.
- Confirm, cancel, expire and supersede decisions create one immutable receipt
  bound to the exact Proposal digest and Incident ID. A terminal Proposal
  without that receipt cannot regain execution authority after migration or
  restart.
- Credential-shaped arguments, tool/action mismatch, duplicate identity,
  expiry, drift and replay fail closed.

### P7-HT2-B - Mission, Team and RoundTable entry tools

`CURRENT / SOURCE AND LOCAL INSTALL GREEN`

- `loom.missions.create.preview`
- `loom.missions.continue.preview`
- `loom.teams.create.preview`
- `loom.roundtables.open.preview`

Confirmation records the decision in the encrypted Conversation and only then
opens the existing macOS review flow. It does not create a Mission, Team,
RoundTable Session, Run or Attempt by itself.

### P7-HT2-C - Read and governance tools

`CURRENT / SOURCE AND LOCAL INSTALL GREEN`

The admitted Registry contains 13 bounded metadata tools:

- `loom.sessions.search`
- `loom.missions.search`, `loom.missions.status`
- `loom.teams.search`, `loom.teams.status`
- `loom.roundtables.status`
- `loom.governance.needs_you`
- `loom.runtimes.status`, `loom.providers.status`
- `loom.diagnostics.incident`
- `loom.workspace.status`, `loom.conversation.route.status`
- `loom.library.search`

It also contains 15 mutation-preview tools: Session alignment; Mission create
and continue; Team create and edit; RoundTable open, pause, steer, retry, skip
and replace; Conversation Route, model and reasoning changes; and workspace
selection. Every one produces a Proposal. None can execute or confirm itself.

Stop is an explicit non-tool product decision. It remains an immediate local
operator control and is intentionally absent from the model-facing Registry so
a model can neither delay nor invoke it.

### P7-HT2-D - Cross-Runtime adapters

`ACCEPTED / SOURCE GREEN / BUILD 324 AVAILABLE-RUNTIME LIVE GREEN / CLAUDE LIVE N/A`

Codex, OpenCode, Claude Code, Pi and Loom Native must expose the same admitted
registry for one frozen turn. Adapter-specific transport may differ, but tool
identity, schemas, call limits, Proposal validation and user authority may not.

Source parity now covers all five adapters. OpenAI-compatible and Anthropic
transports use bounded exact tool loops; Codex and OpenCode use the private
owner-loopback MCP caller; Pi uses its typed control bridge; Loom Native uses
the Segment control loop. All routes freeze the same Registry digest, Route,
Workspace, Segment, Attempt and Incident identity before dispatch.

Build 261 source additionally proves that Pi is offered as an executable
Conversation Route only when its Runtime and integrity-checked local model
backend are both available. Local App bootstrap discovers the established
locked model location without duplicating path policy in Swift; the macOS model
catalog retains the exact Qwen model. Missing or changed assets keep the Pi
Route absent while the rest of Loom remains available.

Build 262 strengthens this gate by pinning the official llama.cpp executable
SHA-256 as well as the Qwen GGUF SHA-256. Discovery and process start reject an
unknown digest, and the exact file identities are revalidated immediately
before execution.

Build 263 makes that two-level state explicit in both Runtime surfaces. A
discovered Pi Harness may remain online for non-Conversation capabilities, but
when its locked model backend is absent the App renders `Local model required
for Conversation` and does not advertise capacity as if the Route were
executable.

Build 264 closes two authority and continuity gaps without changing that Pi
gate. Terminal Proposal decisions now persist exact immutable receipts and
surface their Incident ID in the macOS review. Segment execution bindings stay
frozen, while second and later Turns validate the current Attempt Capsule
instead of incorrectly requiring the Segment-opening Capsule digest.

Build 265 makes the official llama.cpp release archive, not its 33KB launcher
alone, the local Runtime trust root. The exact source-pinned archive is parsed
with bounded path/link/type rules into a deterministic complete tree. Every
server binary, sibling dylib, link alias and metadata file must be materialized
as an owner-only, non-symlink, single-link regular file with matching bytes;
directories are owner-only. Extra files, dependency drift, traversal, archive
substitution and identity changes fail closed before Route publication and are
rechecked before process start. The daemon accepts local model state only as a
complete private-root/archive/server/model tuple. Build 265 is a deterministic,
signed historical candidate; it did not replace installed Build 264 at that
checkpoint.

Installed Build 313 carries the complete Pi assets and the current model-tool
loop. Argument repair is bounded, exhausted repair forces a fresh terminal
selection, and a model cannot execute a Tool omitted from the current
least-privilege round. A transient metadata Gateway outage receives one retry
only for read effects; Proposal effects never auto-retry. The installed
acceptance fixture selects an explicit governance anchor and creates synthetic,
`p7-`-prefixed RoundTables so each Pause, Steer, Retry, Skip and Replace Proposal
is validated against action-eligible current state. It never converts a
concluded historical Session into writable authority.

Installed Build 315 adds the bounded Claude native-auth recovery path without
weakening Profile admission. Runtime & Providers can start the official Claude
login through an exact UDS Route, Setup dynamically re-observes auth and can
publish the Profile without restart, and failures remain Claude-local with
stage and Incident metadata. A detected but logged-out Runtime is rendered as
`Sign In Required`, not executable. Build 315 does not claim the Build 313 paid
matrix was rerun.

Installed Build 316 closes the review findings around that recovery path. One
central Setup admission gate now prevents any credential, Vault, Provider,
Team or Runtime refresh from replacing a known-good inventory with a transient
empty projection. The App generates one Incident ID for Claude login and binds
it to the exact UDS request, safe staged diagnostics, timeout and user-visible
recovery state. Runtime & Providers exposes bounded cancellation while polling;
cancel clears presentation without fabricating a failure. Additional process
tests fail closed on Home or temporary-directory identity replacement and prove
descendant process-group cleanup. No credential or Claude Profile was created
during this install gate.

Installed Build 317 closes the remaining cancellation-ordering gap. Swift UDS
calls are cancellable at the live descriptor, so cancelling a blocked Setup
poll cannot delay the exact daemon cancel request. Claude sign-in uses one task
generation, keeps the in-flight state while cancellation is pending and rejects
all stale publication. The macOS row distinguishes waiting from cancelling,
and no non-Claude Runtime inherits that state. Real strict Go-to-Swift protocol
probes now use debug compilation for bounded source-test latency; installed App
packaging remains an independent deterministic release build. This install gate
did not initiate authentication or make a paid Provider request.

Build 318 installs the post-Build 317 hardening that makes final admission
independent of display labels and generic adapter availability. Claude Profile publication now
requires the canonical Runtime instance, exact adapter and model, positive
capacity and `workspace_edit`. The live runner accepts only the exact static or
account-scoped Profile identity, protocol, auth mode, Provider Account and
credential revision contract for every Harness. Its restart privacy gate scans
the exact private state and diagnostic roots for a content-only marker while
never placing that marker in IDs. A real Swift client/private-UDS/product-daemon
contract proves that a blocked Setup exchange is cancelled before the exact
Incident-bound daemon cancellation is admitted. Deterministic packaging,
transactional installation, cold startup and exact installed identity pass.

### P7-HT2-E - Installed live matrix

`ACCEPTED / BUILD 324 INSTALLED LIVE GREEN / CLAUDE LIVE N/A BY USER`

For varied Chinese and English wording, each installed Runtime must prove
model tool selection, Proposal rendering, user confirmation, exact review
navigation, cancellation, expiry, replay rejection and restart restoration.
No completion claim is allowed from mocked tool calls alone.

The live runner defaults to skip and requires explicit operator gates for real
paid requests and managed App restart plus exact installed build, App
executable SHA-256 and bundled daemon SHA-256 identity. It rechecks strict
signing and the same file identity after restart. Governance fixtures require
one unambiguous P7 acceptance Mission or explicit exact Mission/Team IDs;
partial Session creation registers cleanup immediately. The runner prepares
three synthetic Mission-draft turns per Runtime, partitions the 28 Tools into
exact single-Tool ordinary-language turns and verifies confirmation with replay
rejection, cancellation, five-minute expiry and encrypted restart restoration.
It changes no credential, Mission or Team state. Synthetic RoundTable execution
state is `p7-` prefixed, action-valid and concluded before completion.

The explicitly authorized Build 313 available-Runtime gate passed for Codex,
OpenCode, Pi and Loom Native. On 2026-08-30 the user stated that Claude Code is
unavailable and explicitly waived only its installed login and paid live call.
The waiver does not remove Claude source parity or fail-closed Profile admission.
The available-Runtime gate requires a separate explicit waiver flag
and refuses to bypass Claude if an executable Claude Profile is present.

The authorized Build 318 rerun failed safely on its first Codex turn because the
installed Codex refresh token was invalid while the legacy native `login status`
probe still returned success. The subsequent source correction actively probes
the official App Server `account/read`, publishes no Codex Profile while
reauthentication is required, invalidates cached auth state on reconnect, and
maps a mid-Session unauthorized notification to a privacy-safe, actionable
`provider_auth` failure.

Build 323 then completed the four-Runtime tool and restart sequence but failed
the final privacy gate because Codex/OpenCode child CLIs had left default-umask
temporary directories under persistent scratch roots. Build 324 gives every
subprocess call a canonical owner-only temporary directory, removes it on every
exit path and migrates legacy scratch trees to directories `0700` and files
`0600` without following links or accepting identity drift.

The explicitly authorized exact Build 324 matrix passed in 407.66 seconds for
Codex, OpenCode/DeepSeek, Pi and Loom Native/MiniMax. It covered all 28 Registry
tools, exact read and Proposal selection, confirmation, cancellation,
digest-bound expiry, replay rejection, action-eligible RoundTable targets,
managed restart, decision restoration and the final bounded privacy scan. A
separate Codex text reply and Mission Proposal probe passed before the matrix.
The installed preflight reports 24 Providers, seven Runtimes and six executable
Profiles. Claude remains discovered but has no executable Profile; the user's
explicit live waiver is accepted only because the gate independently verifies
that absence.

## Current source acceptance

- The versioned Registry admits exactly 28 tools: 13 bounded metadata reads and
  15 mutation previews. Duplicate IDs, unknown fields, excess calls and tool or
  action substitution fail closed.
- Proposal schema v2 binds the exact Route, Workspace, Registry digest,
  Segment, Attempt, Incident, target content, expiry and proposal digest.
  Confirmation rechecks current daemon and thread projections before issuing
  an immutable one-time decision receipt. Cancel, expiry and supersede
  decisions use the same exact receipt contract. Schema v1 remains readable
  and cancellable for migration, but cannot be confirmed or executed.
- Product metadata reads require the current frozen turn, exact Registry
  digest and Incident ID. They return bounded non-secret projections and never
  Prompt, transcript, Provider response, credential or writable authority.
- API tests prove alignment and all action proposals reject expiry, replay,
  workspace/route drift, Registry substitution and target mismatch. Mission
  draft confirmation creates no Mission, Run or Attempt.
- Codex, OpenCode, Claude Code, Pi and Loom Native pass the same read and
  Proposal parity contracts without exposing confirm, cancel or execute tools.
- Setup and the installed runner fail closed independently: a generic online
  adapter cannot publish Claude, a malformed Profile projection cannot satisfy
  the five-Runtime gate, and the available-Runtime gate cannot bypass a real
  executable Claude Profile.
- The final privacy gate uses a content-only marker and bounded owner-only scans
  of state and diagnostics after restart; IDs and operational metadata never
  contain the marker.
- The macOS client strictly decodes both Proposal families, rechecks the exact
  frozen turn and dispatches the 14 action types to their existing governed
  reviews only after daemon confirmation. Alignment retains its dedicated
  immutable Segment flow.
- Restart tests restore only confirmed Proposals with an exact matching
  receipt; historical no-receipt, stale and v1 Proposals remain
  non-executable. Content-free `chat_control_decision` diagnostics correlate
  each decision by Incident ID without recording Conversation content.
- The complete Go repository test suite and `go vet ./...` pass. The latest
  macOS source suite passes 496 XCTest cases with two conditional skips and 21
  strict Swift contract cases; both the v2 Mission Proposal and confirmed
  decision-receipt cards are visually checked at 560px and 360px accessibility
  size.

## Completion gate

The Phase 7 Goal required every capability family to be represented
by an admitted tool or an explicit non-tool product decision, all five Runtime
adapters pass source parity and privacy gates, the macOS App has one
understandable Proposal/intervention experience, and every Runtime available to
the user passes the exact installed live matrix. A user-waived unavailable
Runtime is recorded as N/A, never silently counted as passed.

Build 324 satisfies that gate. All five adapters pass source parity and
privacy contracts; Codex, OpenCode, Pi and Loom Native are available and pass
the exact installed matrix; Claude is unavailable and explicitly recorded as
N/A. Exact bundle identity, signing, managed startup, private UDS inventory,
all 28 Tools, Proposal decisions, expiry, replay rejection, exact action
targets, restart restoration and plaintext/privacy scanning are accepted.
The source evidence is `../verification/P7-HT2-source.md`; current installed
evidence is `../verification/P7-HT2-installed-build324.md`; Build 323's failed
privacy result remains historical evidence at
`../verification/P7-HT2-installed-build323-failed-privacy.md` and is not
represented as a pass.
Build 265 candidate evidence is preserved at
`../verification/P7-HT2-candidate-build265.md`. Build 317 cancellation
evidence remains preserved at `../verification/P7-HT2-installed-build317.md`;
Build 316 recovery evidence remains preserved at
`../verification/P7-HT2-installed-build316.md`, while the
first complete paid four-Runtime matrix remains
`../verification/P7-HT2-installed-build313.md`.
