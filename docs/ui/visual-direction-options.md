 # Loom Visual Design Direction Options

 **Date**: 2026-08-05
 **Scope**: TUI (Bubble Tea) + macOS native app (SwiftUI)
 **References**: Claude Code, OpenAI Codex, Multica
 **Skills**: `ui-ux-pro-max` (dense product UI), `design-taste-frontend` (anti-slop discipline)

 This memo summarizes the visual languages of three reference tools and presents three concrete directions for Loom. Pick one and we will refine the token system, component surfaces, and interaction patterns to match.

 ---

 ## 1. Reference Research

 ### 1.1 Claude Code — "Warm Authority"

 **Source of truth**: Anthropic's `/theme` system, desktop app, and user feedback in `anthropics/claude-code`.

 **Visual feeling**:
 A premium, calm terminal companion. The default theme is dark with a soft, warm gray-black background rather than a harsh void. Text is white and readable. The accent is Claude's signature orange `rgb(215, 119, 87)` used sparingly for headers, status, and brand moments. The overall impression is trustworthy and human-centric — not a machine yelling at you.

 **Key design facts**:
 - Default dark mode is `rgb(255,255,255)` white text on a dark but not pure-black background.
 - Brand accent is warm orange `#D97757`.
 - Semantic colors: success green `#4EBA65`, error red `#FF6B80`, warning yellow.
 - `/theme` supports `dark`, `light`, `dark-daltonized`, `light-daltonized`, `dark-ansi`, `light-ansi`, and custom user themes.
 - 60+ color values cover text, selection, prompts, borders, diff blocks, and agent identifiers.
 - The React-based terminal renderer supports collapsible tool results, multi-line input, and a persistent status line.
 - User feedback praises the *previous* dark theme for "softer, layered grays and muted accents" and criticizes the newer pure-black/bright-blue combination as harsh and less premium.

 **What to borrow for Loom**:
 - Warm, soft dark canvas (not `#000000`).
 - A clear theme system with light/dark/ANSI variants.
 - Semantic color slots (text, muted, success, warning, error, selection, accent) rather than ad-hoc hex.
 - Distinctive brand accent used for headers and status, not every interactive element.

 **Risk for Loom**:
 Orange is Claude's identity. If Loom adopts orange it will read as "Claude-flavored." We should either keep Loom's existing indigo-violet accent or derive a warmer accent from Loom's own brand.

 ### 1.2 OpenAI Codex — "Terminal Native"

 **Source of truth**: `openai/codex` TUI docs (`openai-codex.mintlify.app`), `tui/styles.md`, and PR history.

 **Visual feeling**:
 A fast, no-nonsense coding agent that lives inside your terminal. The UI is intentionally terminal-native: it uses ANSI semantic colors and respects the terminal's own palette. It feels like a well-configured `tmux` or `vim` session rather than a polished GUI. The tone is engineering-first and raw.

 **Key design facts**:
 - Built with **Ratatui** (Rust TUI framework) and **Crossterm**.
 - Uses ANSI semantic colors: `cyan` for user input/selection, `magenta` for Codex identity, `green` for success/additions, `red` for errors/deletions, `dim` for secondary text, `bold` for headers.
 - **Style rule**: avoid custom RGB, avoid ANSI black/white foreground, avoid ANSI blue/yellow.
 - Prompt character changed from cyan `▌` to bold `>`.
 - User messages and composer area have a colored background that stretches the full terminal width.
 - "Working" shimmer follows the dark-gray terminal color.
 - Adapts to 24-bit, 256-color, or 16-color terminals automatically.

 **What to borrow for Loom**:
 - Semantic ANSI color mapping so the TUI degrades gracefully on older terminals.
 - A bold `>` prompt marker as a recognizable Loom signature.
 - Minimal decoration: color is used for meaning, not branding.
 - Full-width background bands for status/composer to create visual anchors without boxes.

 **Risk for Loom**:
 Too strict ANSI-only can feel generic. Loom's governance surfaces (permissions, teams, missions) need stronger hierarchy than a chat stream. Codex's approach works best as the TUI baseline, not the macOS app identity.

 ### 1.3 Multica — "Linear Governance"

 **Source of truth**: `multica-ai/multica` GitHub commits and PRs (`#558`, `#233`, `#1087`).

 **Visual feeling**:
 "Linear, but with agents as first-class citizens." Multica's UI is cool, organized, and operational. It borrows Linear's issue-tracker density: floating white panels on a subtle canvas, sticky headers, 13px metadata, status pills, and a left rail that blends into the background. The feeling is board-room precision rather than creative chaos.

 **Key design facts**:
 - **Layout**: canvas background + floating panel with rounded corners and soft shadow, offset from edges to reveal canvas underneath.
 - **Sidebar**: no border, blends into canvas, workspace switcher at top, icon+label nav items, active item with subtle accent background.
 - **Light palette**: canvas `oklch(0.95 0.002 286)`, sidebar `oklch(0.985 0 0)`.
 - **Dark palette**: canvas `oklch(0.20 0.005 286)`, sidebar `oklch(0.21 0.006 285.823)`.
 - **Font**: Geist Sans + Geist Mono.
 - **Small type sizes**: 12px-13px for labels, metadata, properties.
 - **Components**: PropertyRow (label left, value right), sticky table headers, status pills, emoji icon picker, breadcrumb headers, board/list toggle.
 - **Radius**: 0.625rem (10px).
 - **Density**: high — rows are compact, spacing is tight, every pixel carries information.

 **What to borrow for Loom**:
 - Canvas + floating panel layout for the macOS app.
 - Property-row pattern for mission/permission details.
 - Status pills and 13px metadata hierarchy.
 - Sticky headers and board/list toggles for the mission queue.
 - Sidebar that blends into the canvas rather than a hard separator.

 **Risk for Loom**:
 Multica is web-first (Next.js). Translating its web layout to SwiftUI and a TUI requires adaptation. The 12px-13px sizes can also strain accessibility if not scaled correctly in native macOS.

 ---

 ## 2. Three Direction Options for Loom

 Each option is described by: **Big idea**, **Palette**, **Typography**, **Layout**, **Motion**, **Feeling**, and **Trade-offs**. The current Loom implementation already uses an indigo-violet accent `#5E6AD2`; that choice is neutral enough to work with any of these directions.

 ---

 ### Option A: "Warm Authority" (Claude-inspired)

 **Big idea**: Loom is a trustworthy governance console. The UI feels like a premium developer tool you can stare at for hours: soft dark background, warm neutrals, and a single confident accent.

 **Palette**:
 | Token | TUI (dark) | macOS (light) | macOS (dark) |
 |---|---|---|---|
 | Canvas | `#111113` | `#F7F7F8` | `#131315` |
 | Surface | `#1A1A1E` | `#FFFFFF` | `#1C1C1F` |
 | Raised | `#222226` | `#F2F2F4` | `#242427` |
 | Text primary | `#FFFFFF` | `#1A1A1E` | `#FFFFFF` |
 | Text secondary | `#9A9A9E` | `#5C5C66` | `#9A9A9E` |
 | Text muted | `#6E6E76` | `#8E8E96` | `#6E6E76` |
 | Accent | `#5E6AD2` (keep) | `#5E6AD2` | `#5E6AD2` |
 | Accent muted | `#232538` | `#E6E8FA` | `#232538` |
 | Success | `#4EBA65` | `#2A8C3E` | `#4EBA65` |
 | Warning | `#E5A13A` | `#B0781C` | `#E5A13A` |
 | Danger | `#FF6B80` | `#C72C48` | `#FF6B80` |
 | Border | `#2A2A2F` | `#E2E2E6` | `#2A2A2F` |

 **Typography**: System/SF Pro for UI, SF Mono for data and status. Body 14px, metadata 13px.
 **Layout**: Left rail (192px) with clear active state, main content panel, right inspector on detail screens. Generous but not wasteful padding.
 **Motion**: 0.2s ease for selection and hover. No continuous animations. Selection background changes are the primary motion.
 **Feeling**: Calm, premium, authoritative, long-session comfortable.
 **Trade-offs**: Leans heavily on dark-mode refinement. macOS light mode must be carefully tuned to avoid feeling like a generic settings app. The warm canvas is a deliberate departure from the cool blue-gray of many developer tools.

 **Best for**: Loom if you want to feel like a high-end, opinionated developer product rather than a generic open-source dashboard.

 ---

 ### Option B: "Terminal Native" (Codex-inspired)

 **Big idea**: Loom is an extension of the terminal. The TUI honors the user's terminal palette, and the macOS app stays minimal and system-native. Color is purely semantic.

 **Palette**:
 | Token | TUI | macOS |
 |---|---|---|
 | Canvas | terminal default / `#050506` fallback | `NSColor.windowBackgroundColor` |
 | Surface | terminal bright black / `#121214` | `NSColor.controlBackgroundColor` |
 | Text primary | terminal default foreground | `NSColor.labelColor` |
 | Text secondary | terminal dim / bright black | `NSColor.secondaryLabelColor` |
 | Accent | ANSI cyan `#22D3EE` | system accent color |
 | Success | ANSI green `#4ADE80` | `NSColor.systemGreen` |
 | Warning | ANSI yellow `#FACC15` | `NSColor.systemOrange` |
 | Danger | ANSI red `#FB7185` | `NSColor.systemRed` |
 | Selection | ANSI cyan bg + black fg | system selection color |

 **Typography**: Strict monospace for TUI (JetBrains Mono / SF Mono), system fonts for macOS. Everything is functional.
 **Layout**: TUI is full-screen with full-width status bands. macOS follows native sidebar + table patterns, no floating panels.
 **Motion**: Minimal. Selection changes instantly. 0.1s fade for state transitions only where necessary.
 **Feeling**: Fast, raw, native, predictable, no chrome.
 **Trade-offs**: Less brand personality. The macOS app can look like a system utility. The ANSI-only approach limits the expressive range of the TUI (e.g., no subtle brand purples). Accessibility must be verified on every terminal palette.

 **Best for**: Loom if you want maximum terminal compatibility and a "ships with your shell" identity. Best if Loom is used mostly in the TUI.

 ---

 ### Option C: "Linear Governance" (Multica-inspired)

 **Big idea**: Loom is an operational command center for agent teams. The UI is dense, board-like, and precise: issues/permissions/missions are treated as first-class work items with status, owners, and clear state.

 **Palette**:
 | Token | TUI (dark) | macOS (light) | macOS (dark) |
 |---|---|---|---|
 | Canvas | `#0A0A0C` | `#F5F5F7` (canvas) | `#17181A` (canvas) |
 | Panel | `#121214` | `#FFFFFF` (floating panel) | `#222326` (floating panel) |
 | Rail | `#0D0D0F` | `#F0F0F3` (blends into canvas) | `#1A1B1E` |
 | Text primary | `#EAEAEC` | `#1F2328` | `#E8E8EA` |
 | Text secondary | `#8B949E` | `#656D76` | `#8B949E` |
 | Text muted | `#52545A` | `#8C959F` | `#52545A` |
 | Accent | `#5E6AD2` (keep) | `#5E6AD2` | `#5E6AD2` |
 | Accent muted | `#1E2035` | `#E7E9FC` | `#1E2035` |
 | Success | `#22C55E` | `#16A34A` | `#22C55E` |
 | Warning | `#F59E0B` | `#D97706` | `#F59E0B` |
 | Danger | `#EF4444` | `#DC2626` | `#EF4444` |
 | Border | `#1E1E22` | `#E2E4E8` | `#2A2C30` |
 | Shadow | none | `0 4px 24px rgba(0,0,0,0.06)` | `0 4px 24px rgba(0,0,0,0.35)` |

 **Typography**: Geist Sans / SF Pro for UI, Geist Mono / SF Mono for metadata. 13px for property labels, 14px for body, 15px for navigation, 12px for micro labels.
 **Layout**: macOS uses floating panels on a canvas — left sidebar blends in, main content is a rounded white panel with shadow and subtle border. TUI uses dense list rows with status pills and sticky section headers. Right inspector shows mission/permission properties as `PropertyRow` (label left, value right).
 **Motion**: 0.15s ease for hover states, 0.2s for selection, sticky headers appear as panels scroll. No heavy motion.
 **Feeling**: Organized, operational, precise, board-room. The UI says "we are tracking work, not chatting."
 **Trade-offs**: The floating-panel web layout is harder to translate faithfully to SwiftUI. The high density can feel cramped on small screens. The cool neutral palette is safe but may feel less distinctive than the warm authority direction.

 **Best for**: Loom if the core metaphor is "issue tracker for agent governance" and the user spends most of their time triaging missions, permissions, and approvals.

 ---

 ## 3. Synthesis & Recommendation

 **Current state**: Loom already has an indigo-violet accent `#5E6AD2` and a dark-first TUI. The macOS app uses system-adaptive colors but lacks a strong layout identity.

 **My recommendation**: Start with **Option C (Linear Governance)** for the macOS app because it directly addresses the biggest structural problem (monolithic 2,435-line workbench, weak hierarchy, no operational density). At the same time, adopt **Option A's warm dark palette and theme system** for the TUI to improve long-session comfort and brand coherence. Borrow **Option B's semantic ANSI discipline** for TUI fallback themes only.

 In other words:
 - **TUI**: Warm dark canvas + indigo-violet accent + semantic status colors + theme variants (dark/light/ANSI).
 - **macOS**: Linear-style floating-panel layout + property rows + status pills + blended sidebar + indigo-violet accent.
 - **Shared**: `#5E6AD2` accent, 44pt action targets, 12px radius, 0.2s motion, no generic AI-purple.

 This gives Loom a distinctive, coherent identity: the TUI feels like a premium governance console, and the macOS app feels like a mission-critical operations board.

 ---

 ## 4. Next Step

 Choose one of the three directions (or the mixed recommendation). Once you pick, I will:
 1. Fix the current failing TUI test (`Global:` vs `Keys:` label).
 2. Complete the token and component refactor for the chosen direction.
 3. Add loading/empty states, hover/pressed feedback, and accessibility verification.
 4. Run the final test suite (`go test ./internal/tui/...`, `swift test`).


## 5. Implemented: Multica-style Mission Board (macOS)

**Chosen direction**: Option C (Linear Governance) for the macOS Mission Board, with Loom's existing indigo-violet accent `#5E6AD2`.

**Changes in `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`**:
- Added `MissionLaneHeader`, `MissionStatusBadge`, `MissionPriorityBadge`, `LoomCardButtonStyle`, and `boardToolbarTabs` helpers.
- Lane headers now show a colored status dot, lane name, count badge, and a `+` add button.
- Lanes widened from 168px to 280px with a subtle surface background and border.
- Cards now show: short mission ID, status badge, title, description/last milestone, priority badge, attention indicator, and node progress.
- Toolbar styled as a tab bar (Board/Topology/Timeline/Capacity), mission count, filter, and a prominent "+ New Mission" button.
- Added pressed feedback on cards via `LoomCardButtonStyle` (subtle scale down).

**Verification**:
- `swift test` passed (101 tests, 0 failures).
- `go test ./internal/tui/...` passed.
- Screenshot captured from the test harness showing the board with a sample mission in the Orchestrating lane.

**Screenshot**: `/tmp/loom-screenshots/mission-board.png`
