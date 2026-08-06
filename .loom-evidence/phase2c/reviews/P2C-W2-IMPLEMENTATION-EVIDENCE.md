 # P2C-W2 Implementation Evidence — Chat-First Conversation
 
 **WorkItem**: P2C-W2 — Chat-First Conversation  
 **Contract**: `.loom-evidence/phase2c/contracts/P2C-W2-CONTRACT.md`  
 **Date**: 2026-08-06  
 **Status**: implementation complete — pending whole-WorkItem review and atomic commit
 
 ## What changed
 
 1. **Go daemon chat backend** (`internal/api/local_product_chat.go`):
    - In-memory `LocalProductChatAPI` with thread isolation and 4 096-character content bound.
    - Plain messages return `loom` role; explicit agent triggers (`use agent`, `agent team`, `team` + `mission`) return a `proposal` role and a tentative prompt to confirm.
    - No Journal writes; no Team, Mission, or Run facts are created by chat.
 
 2. **IPC wiring** (`cmd/loomd/product_daemon.go`, `internal/api/local_product_read.go`, `internal/localipc/protocol.go`):
    - `chat_thread` and `chat_message` methods added to the local IPC handler.
    - `LocalProductReadService` exposes `ReadChatThread` / `SendChatMessage` backed by the chat API.
    - Invalid chat requests map to `invalid_request`; unavailable chat maps to `state_unavailable`.
 
 3. **macOS chat surface** (`apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift`, `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`, `LocalIPCClient.swift`, `LocalProductModels.swift`):
    - New `LocalProductChatMessage`, `LocalProductChatThread`, and request models.
    - Store methods `loadChatThread` and `sendChatMessage` bound to the current continuity thread anchor.
    - Composer send button now sends a chat message; the welcome card still offers `Use Agent Team` as an explicit trigger.
    - Center workspace renders a real message timeline when messages exist; otherwise it shows the welcome/recent cards.
 
 4. **TUI chat surface** (`internal/tui/model.go`, `style.go`):
    - Added `ChatClient` interface and `DaemonReadClient` implementations.
    - `ScreenHome` is now the first screen and renders a chat-first welcome with message history and draft hints.
    - `i` starts a draft, `enter` sends, `u` starts a Team Draft, `g b` opens the Board.
    - Chat messages are loaded after every snapshot refresh.
 
 5. **RED tests**:
    - `internal/api/local_product_chat_test.go`: plain message does not require confirmation; explicit trigger returns a tentative proposal; empty content rejected; threads are isolated.
    - `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`: `testPlainChatMessageDoesNotCreateTeamOrMission` and `testExplicitAgentTriggerRequiresConfirmationAndDoesNotAutoCreateTeam`.
    - `internal/tui/model_test.go`: updated to expect `ScreenHome` as the default and to navigate to the Board for mission-specific workflows.
    - `cmd/loomd/product_daemon_test.go`: updated headless TUI assertion to accept the Home-first safe view before navigating to the Board.
 
 ## Verification
 
 | Check | Command | Result |
 |---|---|---|
 | Swift build | `cd apps/macos && swift build` | PASS |
 | Swift tests | `cd apps/macos && swift test` | 103 tests, 0 failures, 1 skipped |
 | Go API tests | `go test ./internal/api/...` | PASS |
 | Go TUI tests | `go test ./internal/tui/...` | PASS |
 | Go localipc tests | `go test ./internal/localipc/...` | PASS |
 | Go cmd/loom tests | `go test ./cmd/loom/...` | PASS |
 | Go daemon tests | `go test -timeout 120s ./cmd/loomd/...` | PASS |
 
 ## Known remaining work (P2C-W2 → W3)
 
 - The center chat timeline is a simple text list; richer cards (proposal/confirmation/milestone) and streaming markers will be refined in P2C-W3.
 - File/Folder picker and `Open Folder…` are still placeholders from P2C-W1; P2C-W3 will wire them or remove the false affordance.
 - Governance panel consolidation (Decisions/Evidence first-class, inactive sidebar remnants) is the W3 scope.
 
 ## Exclusions honored
 
 - No Event Journal, one-writer, Projection, Scheduler, policy, Grant, or daemon authority change.
 - No implicit Agent intent detection from free text; only explicit triggers (`Use Agent Team`, `u`, builder flow) create Team Draft proposals.
 - No new credential capture or Provider negotiation in the client.
 - No public network, cloud sync, Web UI, or Tauri.
