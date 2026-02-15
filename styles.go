// styles.go - Lipgloss style definitions for the bubbletea TUI
// All styles are rebuilt dynamically when the theme changes.
package main

import "github.com/charmbracelet/lipgloss"

// ─── Color Aliases (set by rebuildStyles) ───────────────────────────────────────

var (
	accent      lipgloss.Color
	accentLight lipgloss.Color
	subtle      lipgloss.Color
	dimmed      lipgloss.Color
	highlight   lipgloss.Color
	surface     lipgloss.Color
	runningClr  lipgloss.Color
	stoppedClr  lipgloss.Color
	suspendClr  lipgloss.Color
	deletedClr  lipgloss.Color
)

// ─── Title / App ───────────────────────────────────────────────────────────────

var (
	titleBarStyle     lipgloss.Style
	titleVMCountStyle lipgloss.Style
	titleLiveStyle    lipgloss.Style
)

// ─── Table ─────────────────────────────────────────────────────────────────────

var (
	tableHeaderStyle     lipgloss.Style
	tableCellStyle       lipgloss.Style
	tableSelectedCellStyle lipgloss.Style
	tableCursorStyle     lipgloss.Style
	tableEmptyStyle      lipgloss.Style
	tableBorderStyle     lipgloss.Style
	tableColDivStyle     lipgloss.Style
	tableHeaderDivStyle  lipgloss.Style
)

// ─── Footer ────────────────────────────────────────────────────────────────────

var (
	footerKeyStyle  lipgloss.Style
	footerDescStyle lipgloss.Style
	footerStyle     lipgloss.Style
	footerSepStyle  lipgloss.Style
)

// ─── Filter ────────────────────────────────────────────────────────────────────

var (
	filterActiveStyle   lipgloss.Style
	filterInactiveStyle lipgloss.Style
	filterIconStyle     lipgloss.Style
)

// ─── Modal / Overlay ───────────────────────────────────────────────────────────

var (
	modalStyle      lipgloss.Style
	modalTitleStyle lipgloss.Style
	modalTextStyle  lipgloss.Style
	errorTitleStyle lipgloss.Style
)

// ─── Info / Viewport ───────────────────────────────────────────────────────────

var (
	infoKeyStyle    lipgloss.Style
	infoValStyle    lipgloss.Style
	infoBorderStyle lipgloss.Style
)

// ─── Spinner / Loading ─────────────────────────────────────────────────────────

var (
	spinnerStyle    lipgloss.Style
	loadingMsgStyle lipgloss.Style
)

// ─── Form Styles ───────────────────────────────────────────────────────────────

var (
	formTitleStyle        lipgloss.Style
	formLabelStyle        lipgloss.Style
	formActiveLabelStyle  lipgloss.Style
	formValueStyle        lipgloss.Style
	formButtonStyle       lipgloss.Style
	formActiveButtonStyle lipgloss.Style
	formHintStyle         lipgloss.Style
)

// ─── Snapshot / Mount List ─────────────────────────────────────────────────────

var (
	listItemStyle         lipgloss.Style
	listSelectedItemStyle lipgloss.Style
	listHeaderStyle       lipgloss.Style
	detailPanelStyle      lipgloss.Style
	detailKeyStyle        lipgloss.Style
	detailValStyle        lipgloss.Style
)

// ─── Build / Rebuild ───────────────────────────────────────────────────────────

func init() {
	rebuildStyles()
}

func rebuildStyles() {
	t := currentTheme()

	// Color aliases
	accent = t.Accent
	accentLight = t.AccentLight
	subtle = t.Subtle
	dimmed = t.Dimmed
	highlight = t.Highlight
	surface = t.Surface
	runningClr = t.Running
	stoppedClr = t.Stopped
	suspendClr = t.Suspended
	deletedClr = t.Deleted

	// ── Title ──
	titleBarStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(accent).
		Padding(0, 1)

	titleVMCountStyle = lipgloss.NewStyle().
		Foreground(accentLight).
		Background(accent)

	titleLiveStyle = lipgloss.NewStyle().
		Foreground(runningClr).
		Background(accent).
		Bold(true)

	// ── Table ──
	tableHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		PaddingRight(1)

	tableCellStyle = lipgloss.NewStyle().
		Foreground(t.Text).
		PaddingRight(1)

	tableSelectedCellStyle = lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		PaddingRight(1)

	tableCursorStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	tableEmptyStyle = lipgloss.NewStyle().
		Foreground(subtle).
		Italic(true).
		PaddingLeft(3)

	tableBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent)

	tableColDivStyle = lipgloss.NewStyle().
		Foreground(dimmed)

	tableHeaderDivStyle = lipgloss.NewStyle().
		Foreground(accent)

	// ── Footer ──
	footerKeyStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	footerDescStyle = lipgloss.NewStyle().
		Foreground(subtle)

	footerStyle = lipgloss.NewStyle().
		PaddingLeft(1)

	footerSepStyle = lipgloss.NewStyle().
		Foreground(dimmed)

	// ── Filter ──
	filterActiveStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	filterInactiveStyle = lipgloss.NewStyle().
		Foreground(subtle)

	filterIconStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	// ── Modal ──
	modalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 3).
		MaxWidth(80)

	modalTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		MarginBottom(1)

	modalTextStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted)

	errorTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(stoppedClr)

	// ── Info ──
	infoKeyStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	infoValStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted)

	infoBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2)

	// ── Spinner ──
	spinnerStyle = lipgloss.NewStyle().Foreground(accent)

	loadingMsgStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted).
		MarginLeft(1)

	// ── Forms ──
	formTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		MarginBottom(1)

	formLabelStyle = lipgloss.NewStyle().
		Foreground(subtle)

	formActiveLabelStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	formValueStyle = lipgloss.NewStyle().
		Foreground(highlight)

	formButtonStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(surface).
		Padding(0, 2)

	formActiveButtonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(accent).
		Bold(true).
		Padding(0, 2)

	formHintStyle = lipgloss.NewStyle().
		Foreground(subtle).
		Italic(true)

	// ── Lists ──
	listItemStyle = lipgloss.NewStyle().
		PaddingLeft(2)

	listSelectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(highlight).
		Bold(true)

	listHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		PaddingLeft(1).
		MarginBottom(1)

	detailPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(1, 2)

	detailKeyStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	detailValStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted)
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func stateColor(state string) lipgloss.Color {
	switch state {
	case "Running":
		return runningClr
	case "Stopped":
		return stoppedClr
	case "Suspended":
		return suspendClr
	case "Deleted":
		return deletedClr
	default:
		return subtle
	}
}

// stateIcon returns a colored dot indicator for VM state.
func stateIcon(state string) string {
	clr := stateColor(state)
	dot := "●"
	switch state {
	case "Deleted":
		dot = "○"
	case "Suspended":
		dot = "◉"
	}
	return lipgloss.NewStyle().Foreground(clr).Render(dot)
}
