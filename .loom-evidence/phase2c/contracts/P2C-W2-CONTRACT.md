
 # P2C-W2 Contract — Chat-First Conversation
 
 **Status**: Repair 2 contract reviewed; acceptance journey rerun pending
 **Date**: 2026-08-06  
 **Owner**: Phase 2C Candidate writer  
 **Parent**: `PHASE-2C-EXIT-CONTRACT.md`
 
 ## Contract
 
 Implement the center conversation surface: a composer, message timeline, and
 explicit transition from ordinary chat to Agent mode. The conversation must not
 become a command authority; it surfaces daemon-provided messages and records
 user intent.
 
 ## Scope
 
 - macOS: center conversation view, composer, message cells, tentative output
   cards, proposal/confirmation cards, modal/pinned decision surfaces.
 - TUI: composer band, message list, status bands, chat-mode state machine.
 - Shared: daemon IPC methods for chat input, mode intent, Team Draft proposals,
   confirmation actions, and message projection.
 
 ## Required Behavior
 
 1. The composer is always visible and focused by default; supports multi-line
    input and a clear send action.
 2. The timeline shows user messages, Loom responses, tentative output,
    milestones, and error/recovery cards.
 3. Ordinary messages stay in chat mode; no Team, Mission, or Run is created.
 4. An explicit trigger transitions to Agent mode: user clicks “Use Agent Team,”
    selects a Team from the left rail, or types a recognized explicit command
    that the daemon confirms with a dialog.
 5. In Agent mode, the right governance panel auto-opens (or remains open) and
    shows the Team Draft / Mission / Timeline.
 6. Main Agent messages are labeled as proposals/drafts; structured fields
    (Team members, Runtime, budget, tasks) render as cards.
 7. User confirmation is required before any Journal fact is created.
 8. Streaming/tentative output is visibly marked and does not overwrite
    authoritative state until the daemon commits it.
 9. The conversation history is bounded; long-running tasks collapse older
    milestones.
 10. TUI uses a full-width composer band with ANSI semantic colors; macOS uses
     the panel design system.
 
 ## Acceptance
 
 - A plain chat exchange creates no Mission or Team facts in the Journal.
 - An explicit “use Agent” flow creates a confirmed Team Draft / Mission after user
   confirmation.
 - TUI and native app render the same message timeline from the same daemon
   fixture.
 - Composer handles empty/whitespace input gracefully.
 - Keyboard shortcuts for send and cancel are discoverable and consistent across
   clients.
 - Accessibility labels distinguish user messages, Loom messages, proposals, and
   confirmations.
 
 ## Verification
 
 - Deterministic matrix: Go full/race/vet/tidy/gofmt; Swift full/TSAN/Release.
 - Journeys J4, J5 (chat → Agent → Draft → confirm → Mission → panel).
 - RED tests proving ordinary chat does not create Team/Mission facts.
 - RED tests proving explicit Agent trigger requires confirmation.
 - Independent implementation review, dual-Result review, whole-WorkItem review.
 
 ## Explicitly Out
 
 - Changing Event Journal or daemon execution semantics.
 - Implicit Agent intent detection from free text (only explicit triggers).
 - Credential capture or Provider negotiation.
 - Web UI or Tauri.
 
