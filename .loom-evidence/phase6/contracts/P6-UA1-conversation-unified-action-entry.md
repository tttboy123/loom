# P6-UA1 - Conversation unified action entry

Status: `ACCEPTED / COMPLETE / INSTALLED 0.5.4 BUILD 257`

Date: 2026-08-28

## Goal

Make the Conversation composer Loom's single discoverable entry for ordinary
chat and product actions. Typing `/` opens a searchable command menu. A bounded
set of explicit Chinese and English natural-language intents resolves through
the same typed action catalog. Unknown slash commands never reach a Provider.

## Codex reference

Local inspection uses the installed Codex desktop bundle
`com.openai.codex`, version `26.818.41509`. Its shipped resources expose:

- a `/`-opened, searchable Slash command menu;
- title, description, input placeholder and empty-result states;
- contextual unavailable explanations;
- separate Skills grouping;
- typed commands including Review, Rename, Side, Star, Status and Usage;
- a separate `@` context-entry model rather than mixing attachments with
  commands.

Loom adopts the discoverability, filtering and keyboard model. It does not copy
Codex's single-Agent authority boundary or allow command strings to bypass Loom
governance.

## Product contract

1. `/` opens commands in place above the composer; typing filters without
   changing layout dimensions unpredictably.
2. Up/Down moves one stable selection, Return selects, Escape closes, and mouse
   selection is equivalent.
3. Commands are grouped as Conversation, Work, Governance and Setup.
4. The first installed catalog covers Help, New Task, Mission, Missions, Team,
   RoundTable, Continue Mission, Route, Model, Reasoning, Status, Diagnostics,
   Stop, Needs You, Library, Runtime & Providers, Folder and Copy Conversation.
5. `/mission objective` and `/team purpose` retain bounded arguments. Unknown
   commands remain in the composer with suggestions and are never dispatched as
   chat.
6. Explicit Chinese and English action phrases resolve through the same catalog.
   Questions, explanations and incidental nouns remain ordinary conversation.
7. Navigation and safe local inspection may open immediately. Mission, Team,
   RoundTable execution, continuation, route trust transitions and every other
   governed mutation retain their existing Preview/Review/Confirm boundary.
8. Credential values are never accepted as command arguments. `/providers`
   opens the protected Runtime & Providers surface; it cannot configure a
   credential from chat text.
9. Model output remains Proposal/Candidate only and cannot synthesize a command
   with execution authority.
10. Action selection and feedback contain no Prompt, transcript, Provider body,
    secret or hidden reasoning.

## Public test seams

- `ConversationActionCatalog`: stable definitions, filtering and aliases.
- `ConversationActionRouter`: slash parsing, unknown-command handling and
  conservative natural-language intent recognition.
- `ConversationActionAvailability`: contextual disabled reasons.
- `LoomWorkspaceShell`: command menu rendering, keyboard actions and dispatch
  into existing governed UI paths.
- Installed App: desktop/compact interaction, restart and local-service failure
  recovery without Provider dispatch for local commands.

## Accepted implementation

- `ConversationActions.swift` owns the closed catalog, search, aliases,
  contextual availability, Slash parser, bounded arguments and conservative
  natural-language resolver.
- `ConversationActionPalette.swift` owns the fixed-size grouped command and
  Route/Model/Reasoning choice surfaces.
- `LoomWorkspaceShell.swift` dispatches only typed actions into existing local,
  navigation, proposal, review and bounded-control paths. It does not grant new
  daemon authority.
- Mission and Team arguments prefill governed review drafts. RoundTable opens
  Mission-linked governance. Runtime & Providers remains the only credential
  entry surface.
- Diagnostics preview and exporter are published atomically so the command
  cannot open an empty sheet during asynchronous preparation.

Source acceptance covers 442 XCTest cases with two conditional skips and 20
strict Swift contract cases. The Phase 6 command subset contributes ten focused
tests. Desktop, compact and accessibility rendering is nonblank and bounded.
The full Go repository suite was green before the final Swift-only presentation
fix.

Installed Loom `0.5.4` Build 257 passed strict signature and candidate/install
digest equality. Live interaction covered search, keyboard selection, unknown
commands, credential-shaped rejection, governed Mission and Team drafts,
RoundTable, Route/Model choice, Status, Diagnostics, unavailable Stop, draft
preservation while opening Commands, compact layout, App restart and automatic
managed-daemon recovery. Detailed evidence is in
`../verification/P6-UA1-installed-build257.md`.

## Non-goals

- no arbitrary plugin command execution;
- no command-defined shell or filesystem authority;
- no credential import through Conversation;
- no model-generated direct action dispatch;
- no replacement of detailed Mission, Team, RoundTable or Provider editors.
