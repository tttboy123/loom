package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Loom design tokens for the TUI. These mirror the macOS LoomGraphite
// palette so the terminal and native app feel like one surface.
// Accent: #5E6AD2 indigo-violet (distinct from default AI purples and
// default system blue). Status colors are high-contrast against dark
// terminal backgrounds.
var (
	loomAccent      = lipgloss.Color("#5E6AD2")
	loomAccentMuted = lipgloss.Color("#232538")
	loomOnAccent    = lipgloss.Color("#FFFFFF")

	loomCanvas  = lipgloss.Color("#050506")
	loomSurface = lipgloss.Color("#121214")
	loomRaised  = lipgloss.Color("#0F0F11")

	loomTextPrimary   = lipgloss.Color("#EDEDEF")
	loomTextSecondary = lipgloss.Color("#8A8F98")
	loomTextMuted     = lipgloss.Color("#52525B")

	loomSuccess = lipgloss.Color("#22C55E")
	loomWarning = lipgloss.Color("#F59E0B")
	loomDanger  = lipgloss.Color("#EF4444")
	loomInfo    = lipgloss.Color("#5E6AD2")
)

// TUI layout styles. Keep changes additive so the underlying text content
// remains unchanged for tests that assert strings.Contains on rendered views.
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(loomAccent).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Foreground(loomTextPrimary).
			Bold(true)

	sectionStyle = lipgloss.NewStyle().
			Foreground(loomTextSecondary).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Background(loomAccentMuted).
			Foreground(loomOnAccent).
			Bold(true)

	selectedMarkerStyle = lipgloss.NewStyle().
				Foreground(loomAccent).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(loomTextMuted)

	statusSuccessStyle = lipgloss.NewStyle().Foreground(loomSuccess)
	statusWarningStyle = lipgloss.NewStyle().Foreground(loomWarning)
	statusDangerStyle  = lipgloss.NewStyle().Foreground(loomDanger)
	statusInfoStyle    = lipgloss.NewStyle().Foreground(loomInfo)
	statusMutedStyle   = lipgloss.NewStyle().Foreground(loomTextSecondary)

	errorStyle = lipgloss.NewStyle().
			Foreground(loomDanger).
			Bold(true)

	warningBannerStyle = lipgloss.NewStyle().
				Foreground(loomWarning).
				Background(loomRaised)

	mutedBannerStyle = lipgloss.NewStyle().
				Foreground(loomTextMuted).
				Background(loomSurface)

	dividerStyle = lipgloss.NewStyle().Foreground(loomTextMuted)
)

// styleStatus colors a status word based on its semantic meaning. It preserves
// the original text so tests that strings.Contains for the plain word still pass.
func styleStatus(status string) string {
	switch strings.ToLower(status) {
	case "succeeded", "success", "complete", "completed", "available",
		"verified", "active", "approved", "green", "ready", "online",
		"connected", "ok", "passed", "pass", "running":
		return statusSuccessStyle.Render(status)
	case "failed", "failure", "error", "danger", "offline", "fatal",
		"rejected", "denied", "rolled_back", "stopped", "degraded",
		"defect", "critical":
		return statusDangerStyle.Render(status)
	case "warning", "warn", "partial", "stale", "pending", "queued",
		"admitted", "claimed", "paused", "attention", "needs_review",
		"review", "recoverable":
		return statusWarningStyle.Render(status)
	case "proposed", "ready_for_review", "orchestrating", "in_progress",
		"loading", "connecting", "unknown":
		return statusInfoStyle.Render(status)
	default:
		return statusMutedStyle.Render(status)
	}
}

// styleHeader renders the Loom screen header with the accent brand color.
func styleHeader(title string) string {
	return headerStyle.Render(title)
}

// styleTitle renders a section title in primary text with emphasis.
func styleTitle(title string) string {
	return titleStyle.Render(title)
}

// styleSection renders a subsection label in secondary text.
func styleSection(label string) string {
	return sectionStyle.Render(label)
}

// styleSelected renders a full selected row with a muted accent background.
func styleSelected(row string) string {
	return selectedStyle.Render(row)
}

// styleSelectedMarker renders the selection chevron only.
func styleSelectedMarker(marker string) string {
	return selectedMarkerStyle.Render(marker)
}

// styleHelp renders key-hint text in a muted tone.
func styleHelp(text string) string {
	return helpStyle.Render(text)
}

// styleError renders an error/status-danger line.
func styleError(text string) string {
	return errorStyle.Render(text)
}

// styleWarningBanner renders a warning/partial/stale banner line.
func styleWarningBanner(text string) string {
	return warningBannerStyle.Render(text)
}

// styleMutedBanner renders a muted informational banner line.
func styleMutedBanner(text string) string {
	return mutedBannerStyle.Render(text)
}

// styleDivider renders a horizontal rule in the muted divider color.
func styleDivider(width int) string {
	return dividerStyle.Render(strings.Repeat("─", width))
}

// screenKeyHint returns the compact key legend for each TUI screen. It is kept
// in one place so all screens share a consistent footer vocabulary.
func screenKeyHint(screen Screen) string {
	switch screen {
	case ScreenHome:
		return "i draft · enter send · o folder · u Agent Team · ] governance · p pin · r refresh · q quit"
	case ScreenBoard:
		return "/ filter · enter open · g b Board · g t Mission · r refresh · q quit"
	case ScreenNewMission:
		return "enter edit · t Team · w Work type · p preflight · s start · esc cancel"
	case ScreenMission:
		return "esc Board · r refresh · q quit"
	case ScreenTeamBuilder:
		return "n new · enter answer · c confirm · m/s roles · e/p edit · g credential · esc cancel"
	case ScreenAssets:
		return "/ search · n local · i import · 1-4 templates · e/a/t/z/b/v/x/h/d/u · ] next · r refresh"
	case ScreenTeams:
		return "j/k select · enter open · r refresh · q quit"
	case ScreenRuns:
		return "j/k select · enter compare · r refresh · q quit"
	case ScreenEvidence:
		return "r refresh · q quit"
	case ScreenCompare:
		return "r refresh · q quit"
	case ScreenAttention:
		return "j/k select · r refresh · q quit"
	case ScreenTimeline:
		return "r refresh · ] next page · q quit"
	case ScreenQueue:
		return "n create job · j/k select · r refresh · q quit"
	case ScreenWorkers:
		return "c claim · t test · v review · r refresh · q quit"
	case ScreenIntegration:
		return "i integrate · a adopt · b rollback · r refresh · q quit"
	case ScreenPermissions:
		return "n define · b bind · x validate · d detail · j/k select · r refresh · q quit"
	case ScreenExecution:
		return "p propose · d detail · j/k select · r refresh · q quit"
	case ScreenProduction:
		return "p preview · c confirm · d off preview · x confirm off · r refresh · q quit"
	default:
		return "tab/shift+tab views · r refresh · q quit"
	}
}

// styleKeyHint renders the screen key legend in a muted, dim style suitable for
// a persistent footer.
func styleKeyHint(text string) string {
	return helpStyle.Render(text)
}

// styleLoading renders a loading state line.
func styleLoading(text string) string {
	return lipgloss.NewStyle().Foreground(loomInfo).Render(text)
}

// styleEmpty renders an empty-state line.
func styleEmpty(text string) string {
	return lipgloss.NewStyle().Foreground(loomTextMuted).Italic(true).Render(text)
}
