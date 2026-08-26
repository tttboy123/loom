# Loom

**Conversation first. Governed Agent Teams when the work becomes a Mission.**

Loom is a local-first macOS workspace for pairing with coding Agents and for
governing multi-Agent work. You can begin with an ordinary conversation, then
turn a concrete objective into a titled Mission with an explicit Team,
independent execution bindings, live activity, evidence, accounting and human
intervention.

[Changelog](./CHANGELOG.md) ·
[Current status](./docs/CURRENT.md) ·
[Product plan](./PRODUCT-PLAN.md) ·
[Architecture](./docs/ARCHITECTURE.md) ·
[Development guide](./docs/DEVELOPMENT.md)

> **Release candidate**
>
> This branch prepares Loom `v0.5.3-rc.1` from the installed `0.5.3` Build 188
> checkpoint. Phase 2D is accepted for its defined scope. The four-real-Provider
> Team, real-account revoke/rate-limit matrix and custom-endpoint live loop are
> explicitly deferred; see [Current status](./docs/CURRENT.md).

## Start Here

Open Loom and type in **Conversation**. The App starts and manages its bundled
`loomd` service automatically; users do not need to launch a second process.

1. Start a conversation in the center workspace.
2. Choose a route near the composer when you need a different Harness,
   Provider Account, model or reasoning level.
3. Continue chatting for ordinary pairing work.
4. Choose **Run as Mission** when the work needs a Team, workflow, governed
   execution or durable result.
5. Review the proposed Team and execution bindings, then start the Mission.
6. Follow each Agent's activity, output and status in Mission Room. If a Mission
   is blocked, add guidance through **Continue this Mission**.

If no usable route is available, open **Runtime & Providers**. Loom separates
runtime discovery from Provider credentials, so the screen can tell you whether
the missing piece is a Harness, an account, a model or a credential.

## The Product Model

These five concepts are deliberately separate:

| Concept | What the user should expect |
|---|---|
| **Conversation** | The default pairing surface. It can remain a simple chat and does not create a Team implicitly. |
| **Mission** | One titled, navigable workflow related to a Conversation. Opening it shows that workflow's objective, Team, Agent activity, output, evidence and recovery state. |
| **Agent Team** | A reusable roster and orchestration definition. Every Agent can use a different Harness, Provider Account and model. |
| **Runtime & Providers** | The setup and health surface for Harness runtimes, Provider Accounts, models, credentials and diagnostics. |
| **RoundTable** | A governed collaboration and handoff surface. Seats can be added by drag and drop; it is not the default way to start work. |

The Mission Board is a projection of Missions, not a place where every internal
object is flattened into cards. A Mission title is the entry point; Mission
Room is where the single workflow and its Team execution are understood.

## Conversation

Conversation follows the interaction model users already know from Codex and
Claude Code: the center of the App is the dialogue, while navigation and
governance stay available at the sides.

- A new App session opens into Conversation.
- Multiple conversations are isolated and remain navigable in the sidebar.
- The selected reply shows its actual Harness, Provider Account, model and
  reasoning level.
- The current response can be stopped without deleting the Conversation.
- A Conversation can be copied as a readable transcript.
- A proposal can become a Mission without re-entering its objective.

### Switching routes

A route is an executable combination, not another Provider:

```text
Harness + Provider Account + Credential Revision + Model + Limits
```

Changing route creates an immutable Conversation Segment. Loom does not mutate
the previous Segment or pass a Provider-native session identifier to another
Provider. When context or trust boundaries change, the App asks how to proceed:

- **Continue with context** sends the policy-admitted relevant context.
- **Summary only** sends goals, confirmed constraints, decisions and current
  state with explicit omissions.
- **Start clean** begins with the current request and required policy only.

Context Capsules carry provenance and disclosure metadata. Previous model
output remains untrusted input; hidden reasoning and credentials are never
transferred.

## Missions

A Mission is one execution journey, not a second kind of chat and not a dump of
all project state. It has a title, a link to its source Conversation when one
exists, a Team, a task graph and a sequence of independently audited Attempts.

Mission Room presents:

- the objective and current lifecycle state;
- each Agent's frozen Harness, Provider Account, model and status;
- visible Agent output while execution is in progress;
- node progress, dependencies and failure isolation;
- published results and workspace artifacts;
- evidence, accounting, incidents and recovery actions.

Live Agent output is marked tentative until accepted terminal Evidence exists.
Prompts, secrets, Provider response bodies and hidden reasoning are excluded
from the operational stream.

### Human intervention

Blocked, Failed and Cancelled Missions keep a **Continue this Mission** composer.
Guidance opens a prefilled review for the same Team and starts a fresh Attempt
only after explicit confirmation. The old Attempt, output, failure and Incident
remain unchanged and auditable.

## Agent Teams

Team-level defaults initialize new Agents; they do not overwrite an Agent's
independent binding. The execution chain is:

```text
Team Role
  -> Agent Definition
  -> Execution Profile
  -> Harness Adapter
  -> Provider Account / Credential Revision
  -> Model / Limits / Capabilities
```

The Dispatcher resolves and freezes this chain per Agent and per Attempt. One
Provider Account failure blocks only the affected Agent. Fallback is explicit,
versioned and auditable; Loom does not silently move a Claude or Codex Agent to
another Provider.

## Runtime, Provider And Model

The three selectors answer different questions:

| Dimension | Question | Examples |
|---|---|---|
| **Harness runtime** | How is the Agent executed? | Codex, Claude Code, OpenCode, Loom Native, Pi |
| **Provider Account** | Which model service and credential authority are used? | OpenAI, Anthropic, DeepSeek, Kimi, MiniMax |
| **Model** | Which model does that route invoke? | A model supported by the selected account and Harness |

OpenCode is a Harness runtime. It may execute a DeepSeek or MiniMax route, but
it must not appear as duplicate `opencode-deepseek` or `opencode-minimax`
Provider Accounts. Provider Directory is built from model Providers; Runtime
inventory is built from detected Harnesses; the route menu combines them only
at the point of execution.

The Provider catalog includes official, gateway, cloud, local and custom
compatible descriptors. A catalog entry means Loom understands the setup
contract; it does not claim that the current machine has a credential, a
compatible runtime or a live-verified route. The App shows those states
separately.

## Credentials And Privacy

Normal operation uses the Loom-owned Credential Vault. The macOS Keychain is a
migration source only and is not read for every Conversation turn or Agent
Attempt.

- Secrets enter through the private local IPC boundary only during import or
  replacement.
- The daemon acquires short-lived, exact credential leases by Provider Account,
  reference and revision.
- Credentials do not enter Conversation transcripts, Prompts, Teams, Journal,
  Evidence, diagnostics, process arguments or global environment variables.
- Operational diagnostics contain safe stage, timing, result and Incident IDs,
  not user content or Provider bodies.
- Vault, Conversation and export encryption use separate key domains and fail
  closed on identity, revision or integrity drift.

The default local key-file mode protects against disclosure of the Vault
database by itself. It does not claim protection from an attacker who already
controls the same macOS account, root or daemon memory; use FileVault and normal
device security as part of the threat model.

## Failure And Diagnostics

The UI does not reduce every failure to `Unavailable`. Provider import,
verification, Conversation dispatch and Agent Attempts expose a privacy-safe
stage, retryability, recovery action and Incident ID.

Users can retry, inspect diagnostics or copy the Incident ID without opening a
terminal. Journal facts remain separate from operational diagnostics: the
Journal is execution authority; diagnostics help explain what happened.

## Current Release Boundary

`v0.5.3-rc.1` includes the Phase 2D product boundary and later user-experience
hardening through Build 188:

- App-managed bundled daemon startup;
- Conversation Profiles, Route Segments and Context Capsules;
- Loom Credential Vault and short-lived credential leases;
- independent per-Agent execution bindings;
- Mission creation from Conversation, live Agent activity and result opening;
- actionable blocked-Mission intervention;
- Provider / Runtime / route separation, including OpenCode;
- Incident diagnostics, controlled failure isolation, fallback and accounting;
- governed RoundTable sessions and drag-and-drop seating.

Not claimed by this release candidate:

- one installed Team using four independently credentialed real Providers at
  the same time;
- a live revoke/rate-limit test against a user's real Provider Account;
- a complete real Conversation through a newly imported custom endpoint;
- cloud sync, multi-user collaboration or unattended standing orders.

## Build And Install On macOS

Requirements:

- macOS 14 or newer on Apple silicon;
- Go 1.22 or newer;
- Swift 6;
- one or more supported Harness CLIs for native-runtime routes.

Build a signed local bundle into a new private directory:

```bash
release_root="$(mktemp -d)"
scripts/build-loom-local-app.sh --output "$release_root/Loom.app"
```

Install it transactionally for the current user:

```bash
mkdir -p "$HOME/Applications"
scripts/install-loom-local-app.sh \
  --app "$release_root/Loom.app" \
  --destination "$HOME/Applications/Loom.app"
open "$HOME/Applications/Loom.app"
```

The installer validates ownership, file modes, bundle identity, architecture
and code signature. It keeps one rollback bundle at `Loom.app.previous`.

## Verify A Source Checkout

```bash
go test -p 4 ./... -count=1 -timeout=15m
go test -race ./internal/api ./internal/app ./internal/execution \
  ./internal/localipc ./internal/runtime/...
go vet ./...

swift test --package-path apps/macos
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
git diff --check
```

External Provider tests may spend quota or use real credentials and are not
part of the default source gate. Run only the explicitly scoped live acceptance
scripts with user authorization.

## Architecture In One View

```mermaid
flowchart LR
    U["User"] --> A["Loom macOS App"]
    A -->|"private local IPC"| D["App-managed loomd"]

    D --> V["Credential Vault"]
    D --> J[("Append-only Event Journal")]
    J --> P["Rebuildable projections"]
    P --> A

    D --> G["Harness Gateway"]
    G --> H["Codex / Claude Code / OpenCode / Pi / Loom Native"]
    H --> M["Model Providers"]

    D --> E["Evidence and artifacts"]
    H -. "proposal, never authority" .-> D
```

The daemon is the local authority for policy, credentials, execution and state
transitions. Clients render projections and submit versioned commands; Agent
output cannot approve itself or directly write terminal authority.

## Repository Map

```text
apps/macos/                 Native SwiftUI App
cmd/loomd/                  App-managed local daemon
internal/api/               Local product API
internal/app/               Mission and setup coordination
internal/contextcapsule/    Context admission, packing and disclosure
internal/credentials/       Credential contracts and Loom Vault
internal/harnessgateway/    Segment Sessions and Harness lifecycle
internal/provider/          Provider and model catalogs/adapters
internal/runtime/           Harness adapters and runtime discovery
internal/work/              Mission, Run, Attempt and authority
internal/journal/           Append-only Event Journal
internal/projection/        Rebuildable read models
.loom-evidence/             Contracts and acceptance evidence
```

## Development Rules

Read [AGENTS.md](./AGENTS.md), [Current status](./docs/CURRENT.md) and the
relevant ADR or contract before changing behavior. Preserve these invariants:

1. Ordinary Conversation never creates a Team implicitly.
2. Team drafts and execution require explicit user confirmation.
3. Agent and model output is a proposal, never execution authority.
4. Secrets never enter source control, Prompt, Journal, Evidence or diagnostics.
5. An executor cannot mark its own work done.
6. Historical failed evidence remains historical failure evidence.

Current, source-only, experimental and deferred capabilities must remain clearly
labelled. Tests and installed acceptance outrank roadmap prose.
