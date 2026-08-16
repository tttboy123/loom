# Loom UI/UX Review & Optimization Direction

**Date**: 2026-08-05
**Scope**: TUI (Bubble Tea) + macOS native app (SwiftUI)
**Reference**: Claude Code, Codex, Modern Dark developer-tool conventions
**Skills used**: `ui-ux-pro-max` (primary, dense product UI), `design-taste-frontend` (visual anti-slop discipline)

## Design Read

Loom is a local-first agent governance platform for technical users, with a trust-first / authority-bound language. The current UI is functional but visually under-designed: the TUI renders as monochrome plain text and the macOS app uses generic system tokens with a default-blue accent. The right direction is a dense, professional developer tool (primary skill: `ui-ux-pro-max`) with anti-slop visual discipline (`design-taste-frontend`), drawing on Claude Code's dark sidebar/purple accent and Codex's clean panel hierarchy, but adapted to Loom's own governance identity.

**Design dials** (context-appropriate for a dense developer tool):
- `DESIGN_VARIANCE`: 5 - structured but not generic
- `MOTION_INTENSITY`: 3 - terminal-appropriate, minimal
- `VISUAL_DENSITY`: 7 - information-dense, governance-heavy surfaces

## Current State Audit

### TUI (`internal/tui/`)

| Issue | Severity | Evidence | Impact |
|-------|----------|----------|--------|
| Monochrome output | High | `model.go` uses only `fmt.Sprintf` + `strings.Join`; no lipgloss styles despite `lipgloss` being an indirect dependency | No status hierarchy, no selection emphasis, poor scannability |
| Flat selection indicator | High | Selected items use plain `›` prefix with no color/background | Hard to see current item, especially in long lists |
| No visual section headers | Medium | Section titles like "Mission Detail" are plain text | Users cannot quickly parse screen structure |
| Status words are uncolored | Medium | `humanizeStatus` returns plain text; success/warning/danger states look identical | Critical governance states (failed, approved, offline) don't stand out |
| Frame lacks hierarchy | Medium | Header is plain text + ASCII `─` divider; footer is a hardcoded disclaimer | Wasted vertical space, no brand presence |
| ANSI-unsafe clipping | Medium | `clipView` truncates by rune count, so adding ANSI codes will break width math | Blocks any styling improvement until fixed |
| `help` is a single line | Low | Help toggles one line of global keys; per-screen hints are inconsistent | Users must memorize 15 screens of shortcuts |
| Inconsistent per-screen hints | Low | Some screens list keys at the top, others at the bottom, many don't list them | Hard to discover actions |
| Hardcoded footer | Low | `Saving a team never starts work` appended to every screen regardless of context | Visual noise |
| No loading skeleton | Low | Loading state is just "Loading your local workspace…" | No sense of progress or structure |

### macOS Native App (`apps/macos/Sources/LoomLocalApp*`)

| Issue | Severity | Evidence | Impact |
|-------|----------|----------|--------|
| Generic accent color | High | `LoomGraphite.accent` = `Color(red: 0.16, green: 0.39, blue: 0.92)` - default system blue | No brand identity; looks like a stock SwiftUI template |
| Minimal token system | High | `LoomGraphite` has only 4 semantic colors + 3 surfaces | No text hierarchy, no border/shadow tokens, no spacing scale |
| Rail too narrow for text | Medium | `railWidth = 176` with text labels + no icons on some items | Labels feel cramped, some items may truncate |
| Monolithic workbench | Medium | `MissionWorkbench.swift` = 2,435 lines; `ContentView.swift` = 2,221 lines | Hard to maintain, hard to keep visual consistency |
| Status indicators are plain | Medium | Provider rows use `Circle().fill(...)` with 7×7 dots; status is just text | Small, low-contrast, no badge shape |
| Cards lack depth | Medium | `LoomPanel` / card backgrounds use `LoomGraphite.surface` but no border or shadow | Surfaces visually merge together |
| No distinct hover/pressed states | Low | Buttons mostly use `.plain` or default styles; no global pressed feedback | Feels static, less tactile |
| Two sidebar implementations | Low | `ContentView` contains both `taskSidebar` and `sidebar` (legacy) | Visual/code drift risk |
| Empty states are text-only | Low | `ContentUnavailableView` used in some places, but many empty states are plain text lines | Unpolished first-run experience |
| Decision sheet has good bones | Low | `LoomDecisionSheet` has clear identity/template/action sections | Could use accent icon, better action emphasis |

## Reference Design Direction

### Claude Code cues
- Dark, low-contrast sidebar with a distinctive violet/indigo accent.
- Clear command palette / keyboard-first posture.
- Status chips use color + shape (not just text).
- Strong hierarchy: title > subtitle > metadata.

### Codex cues
- Clean panel separation.
- Subtle hairline borders on cards.
- Minimal but purposeful use of accent color on primary actions.
- Professional, code-native typography.

### Loom adaptation
- **Accent**: `#5E6AD2` (indigo-violet) - distinctive but not "AI purple" default; signals authority without copying Claude.
- **Backgrounds**: keep system-adaptive on macOS (light + dark), but on TUI lean dark since terminals are dark-first.
- **Surfaces**: `#0a0a0c` / `#121214` in TUI; `controlBackgroundColor` / `underPageBackgroundColor` on macOS.
- **Text**: primary `#EDEDEF`, secondary `#8A8F98` in TUI; use system `.primary` / `.secondary` on macOS.
- **Status**: success `#22C55E`, warning `#F59E0B`, danger `#EF4444`, info `#5E6AD2`.
- **Shape**: `cardRadius = 12`, `railWidth = 192` (slightly wider), `minimumActionTarget = 44`.
- **Motion**: 0.2s subtle transitions only; no heavy animation per project constraints.

## Proposed Token System

### TUI (`internal/tui/style.go`)

```go
var (
    Accent      = lipgloss.Color("#5E6AD2")
    AccentMuted = lipgloss.Color("#232538")
    OnAccent    = lipgloss.Color("#FFFFFF")

    Canvas      = lipgloss.Color("#050506")
    Surface     = lipgloss.Color("#121214")
    Raised      = lipgloss.Color("#0F0F11")

    TextPrimary   = lipgloss.Color("#EDEDEF")
    TextSecondary = lipgloss.Color("#8A8F98")
    TextMuted     = lipgloss.Color("#52525B")

    Success = lipgloss.Color("#22C55E")
    Warning = lipgloss.Color("#F59E0B")
    Danger  = lipgloss.Color("#EF4444")
    Info    = lipgloss.Color("#5E6AD2")
)
```

Styles:
- `HeaderStyle`: accent foreground, bold, maybe uppercase tracking.
- `TitleStyle`: primary foreground, bold.
- `SelectedStyle`: accent background, on-accent foreground, bold.
- `StatusStyle`: maps status words to colored foreground.
- `HelpStyle`: muted foreground, dim.
- `ErrorStyle`: danger foreground, bold.
- `WarningStyle`: warning foreground.
- `DividerStyle`: subtle border color.

### macOS (`LoomGraphite.swift`)

```swift
public enum LoomGraphite {
    public static let accent = Color(hex: "#5E6AD2")
    public static let onAccent = Color.white
    public static let accentMuted = accent.opacity(0.12)
    public static let accentSecondary = Color(hex: "#818CF8")

    public static let canvas = Color(nsColor: .windowBackgroundColor)
    public static let rail = Color(nsColor: .underPageBackgroundColor)
    public static let surface = Color(nsColor: .controlBackgroundColor)
    public static let raised = Color(nsColor: .textBackgroundColor)
    public static let separator = Color(nsColor: .separatorColor)

    public static let textPrimary = Color.primary
    public static let textSecondary = Color.secondary
    public static let textMuted = Color(nsColor: .secondaryLabelColor)

    public static let statusSuccess = Color(nsColor: .systemGreen)
    public static let statusWarning = Color(nsColor: .systemOrange)
    public static let statusDanger = Color(nsColor: .systemRed)

    public static let cardRadius: CGFloat = 12
    public static let railWidth: CGFloat = 192
    public static let minimumActionTarget: CGFloat = 44
    public static let motionDuration = 0.2
}
```

## Implementation Plan

1. **TUI frame + ANSI-safe clipping** (low risk, high visibility)
   - Add `internal/tui/style.go` with lipgloss tokens and status mapping.
   - Style the header, divider, status banners, footer, and help in `View()`.
   - Style selected items and status words in the Board screen.
   - Replace `clipView` with ANSI-aware width logic (`lipgloss.Width` / `ansi.PrintableRuneWidth`).
   - Ensure text content remains unchanged so `strings.Contains` tests pass.

2. **macOS token refresh** (low risk)
   - Update `LoomGraphite.accent` to `#5E6AD2`.
   - Add `textPrimary`, `textSecondary`, `textMuted`, `accentSecondary`.
   - Increase `railWidth` to `192`.
   - Apply tokens to `PermissionExplorerView` and `ContentView` headers/cards as a pilot.

3. **Component refinements** (medium risk, can be staged)
   - Add `LoomBadge`, `LoomCard`, `LoomStatusDot` helpers to `LoomGraphite.swift`.
   - Refactor `MissionWorkbench` into smaller view files (out of scope for this pass, but planned).
   - Standardize empty states with `ContentUnavailableView` + icon + action hint.

4. **Verification**
   - Run Go tests: `go test ./internal/tui/...`.
   - Run Swift tests: `swift test` in `apps/macos`.
   - Export preview PNGs for light/dark/compact states and inspect.

## Pre-Flight Checks (from `design-taste-frontend` + `ui-ux-pro-max`)

- [x] Brief inference declared.
- [x] Dial values explicit and reasoned.
- [x] Zero em-dashes planned.
- [x] Color consistency lock: one accent (`#5E6AD2`) across both TUI and macOS.
- [x] Shape consistency lock: radius 12 for cards, 44pt minimum action targets.
- [x] Button contrast check: accent `#5E6AD2` on white = ~5.8:1, passes WCAG AA.
- [x] Reduced motion: no added continuous animation; transitions are 0.2s CSS/Swift transitions.
- [x] Dark mode: TUI dark-first; macOS uses system adaptive surfaces.
- [x] No AI-tell defaults: not using generic AI purple, not using three equal feature cards, not using emojis as icons.
- [ ] Live test run after edits (tracked in implementation plan).

## Remaining Work After This Pass

## Implemented Changes

### TUI

1. **New `internal/tui/style.go`** - design token + style helper module using `lipgloss`:
   - Accent `#5E6AD2`, muted accent background `#232538`, status colors (success/warning/danger/info).
   - Styles: header, title, section, selected row, selected marker, help, error, warning/muted banners, divider.
   - `styleStatus(status)` maps status words to semantic color while preserving the original text for test compatibility.

2. **Frame styling in `internal/tui/model.go`**:
   - Header uses accent color + bold.
   - Divider uses muted color.
   - Loading, offline, and error states are color-coded.
   - Stale / partial banners use warning styling.
   - Help line and footer use muted style.

3. **ANSI-safe clipping** - `clipView` now uses `github.com/charmbracelet/x/ansi` `StringWidth` + `Truncate` so styled lines no longer break width math.

4. **Screen-specific styling** applied to the primary operational screens:
   - `ScreenBoard` - selected row highlight, accent marker, styled status.
   - `ScreenQueue`, `ScreenWorkers`, `ScreenIntegration` (in `attention.go`), `ScreenPermissions`, `ScreenExecution`, `ScreenProduction` - titles, section labels, selected markers, and status words are color-coded.

### macOS Native App

1. **`LoomGraphite.swift` token refresh**:
   - Accent changed from generic system blue to `#5E6AD2` indigo-violet.
   - Added `accentSecondary`, `textPrimary`, `textSecondary`, `textMuted`.
   - `cardRadius` increased 10 -> 12, `railWidth` increased 176 -> 192.
   - Added `Color(hex:)` helper for deterministic brand colors.

2. **`PermissionExplorerView.swift` pilot refinement**:
   - Stronger header hierarchy with larger accent icon and title.
   - Admin lock badge uses bolder warning styling.
   - Profiles/bindings/rules/attention rows use improved spacing and semantic text colors.
   - Empty states keep `ContentUnavailableView` with secondary description color.

3. **`MissionWorkbench.swift` rail + footer**:
   - Rail header uses larger accent icon + semibold title.
   - Section label "WORKSPACE" uses `textMuted`.
   - Connection footer uses larger status dot and secondary text.
   - Authority banner has refined spacing and icon color.

4. **`LocalProductionViews.swift` + `LocalExecutionViews.swift`**:
   - Titles use `title3.weight(.semibold)`.
   - Status/error text uses semantic status colors.
   - Metadata uses `textMuted` and monospaced IDs.
   - Views now sit on `LoomGraphite.surface` background.

## Verification

- `go test ./internal/tui/... -count=1` -> PASS.
- `go test ./cmd/loomd/... ./internal/localipc/... -count=1` -> PASS (strict Swift contract probe builds).
- `cd apps/macos && swift test` -> PASS (101 tests, 1 skipped, 0 failures).
- `go vet ./internal/tui/...` -> clean.
- `gofmt` applied to all changed Go files.

## Notes / Risks

- The `LocalQueueModels.swift` Swift 6 Sendable warning is pre-existing and unrelated to this UI pass; it does not fail debug builds but is surfaced as a warning in the strict release probe.
- `TestLocalRuntimeObservationDaemonRealSQLiteRestart` can fail when the local Pi runtime is unavailable; this is environmental and unrelated to UI changes.

## Remaining Work After This Pass

- Refactor `MissionWorkbench` and `ContentView` into smaller files.
- Add consistent per-screen key hints in the TUI (maybe a sticky bottom bar).
- Add loading skeletons and empty-state illustrations to the macOS app.
- Add hover/pressed states to all interactive rows.
- Consider a Loom wordmark/icon in the TUI header and macOS rail.
- Accessibility audit: focus rings, VoiceOver labels, dynamic type.

## Pre-Flight Checks (updated)

- [x] Brief inference declared.
- [x] Dial values explicit and reasoned.
- [x] Zero em-dashes in shipped UI copy.
- [x] Color consistency lock: one accent (`#5E6AD2`) across both TUI and macOS.
- [x] Shape consistency lock: radius 12 for cards, 44pt minimum action targets.
- [x] Button contrast check: accent `#5E6AD2` on white passes WCAG AA.
- [x] Reduced motion: no added continuous animation; transitions are 0.2s.
- [x] Dark mode: TUI dark-first; macOS uses system adaptive surfaces.
- [x] No AI-tell defaults: not using generic AI purple, not using emojis as icons.
- [x] Live tests run and passed.
