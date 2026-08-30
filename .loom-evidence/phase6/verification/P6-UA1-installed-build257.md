# P6-UA1 installed acceptance - Build 257

Status: `ACCEPTED / COMPLETE`

Date: 2026-08-28

## Bundle identity

- Installed path: `/Users/lune/Applications/Loom.app`
- Version: `0.5.4`
- Build: `257`
- Strict deep code-sign verification: passed
- App executable SHA-256:
  `8a319a506d85a11850940a8af69464badd2c2d56b443ccdcd3f43f79f4d83255`
- Bundled daemon SHA-256:
  `87bfc6e7f96a36b37bda4237880bcd08772b83d5526d62c1844d179d9fae947a`
- Candidate and installed executable digests: equal

## Source gates

- Focused Conversation action suite: 10 tests, 0 failures.
- Complete macOS XCTest suite: 442 tests, 2 conditional skips, 0 failures.
- Strict Swift contract suite: 20 tests, 0 failures.
- Full Go repository suite: passed before the final Swift-only Conversation
  admission and diagnostics presentation fixes.
- Desktop, compact and accessibility command-palette renders: nonblank,
  bounded and without incoherent overlap.

## Installed interaction matrix

1. Typing `/` exposed all 18 grouped actions with contextual unavailable
   reasons. Filtering, Up/Down, Return and Escape worked.
2. `/mis` plus keyboard selection opened Missions without creating chat.
3. `/status` returned the active Route, model and response state after Return.
4. An unknown Slash command showed `Command not found`, preserved the draft
   and created no Conversation message.
5. Synthetic credential-shaped arguments were rejected locally from both
   `/mission` and `/providers`. Neither path opened a governed draft, Provider
   setup or chat dispatch, and the command text remained available to correct.
6. A normal `risk-based review` objective was not mistaken for an `sk-` key and
   opened the expected governed Mission review.
7. `/mission` opened a prefilled, Conversation-linked review with execution
   disabled until explicit context confirmation.
8. `/team` opened a prefilled Agent Team draft. Every Agent's Harness, Provider
   Account, model, limits and fallback remained reviewable, and confirmation
   stayed disabled until the user applied the draft fields.
9. Natural-language `open RoundTable` opened Mission-linked governance and did
   not start a discussion.
10. `/model` opened the bounded model picker; benign extra `/providers` text
    was not retained or interpreted as credential configuration.
11. The Commands icon preserved an ordinary composer draft while opening and
    executing a local action.
12. `/stop` remained visible but disabled without an active response, then
    showed the exact `No response is running` recovery reason.
13. `/diagnostics` displayed App/daemon identity, service health, included safe
    state and explicit exclusions, with Close and user-initiated Export.
14. At approximately 830 px window width, the rail collapsed to icons and the
    command catalog remained scrollable and keyboard-operable beside the
    governance inspector.
15. A full App/daemon restart restored the Conversation, Route and governance
    context; `/status` continued to work with a clean ephemeral command state.
16. Terminating only the managed daemon caused the App to start a new daemon
    automatically in about two seconds. The recovered process retained the
    canonical state, isolation, Socket, Runtime and managed-parent arguments,
    and local commands remained usable.

## Live defect closed

The first Build 255 diagnostics check opened an empty white sheet. The preview
and exporter were published through two independent SwiftUI states, allowing
the sheet item to become visible before its required exporter. Build 256 moved
them into one atomic `ChatDiagnosticPresentation`; final Build 257 retains that
fix and renders the complete privacy-safe diagnostics content.

## Privacy boundary

No real credential was entered, read or changed during Phase 6 acceptance. The
credential-shaped cases used synthetic non-secret values. Evidence contains no
Prompt, transcript, Provider body, Authorization header, API key or hidden
reasoning. Diagnostic export was previewed but not written.
