// styles.go - Lipgloss style definitions for the bubbletea TUI
package main

import "github.com/charmbracelet/lipgloss"

// ─── Color Palette ─────────────────────────────────────────────────────────────

var (
	accent      = lipgloss.Color("#7C3AED") // violet
	accentLight = lipgloss.Color("#A78BFA") // lighter violet
	subtle      = lipgloss.Color("#6C6C6C") // gray
	dimmed      = lipgloss.Color("#4A4A4A") // darker gray
	highlight   = lipgloss.Color("#E8E8E8") // bright white
	surface     = lipgloss.Color("#2A2A2A") // dark surface
	surfaceAlt  = lipgloss.Color("#1E1E1E") // zebra stripe
	runningClr  = lipgloss.Color("#10B981") // green
	stoppedClr  = lipgloss.Color("#EF4444") // red
	suspendClr  = lipgloss.Color("#F59E0B") // amber
	deletedClr  = lipgloss.Color("#6B7280") // muted gray
)

// ─── Title / App ───────────────────────────────────────────────────────────────

var titleBarStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(accent).
	Padding(0, 1)

var titleVMCountStyle = lipgloss.NewStyle().
	Foreground(accentLight).
	Background(accent)

var titleLiveStyle = lipgloss.NewStyle().
	Foreground(runningClr).
	Background(accent).
	Bold(true)

// ─── Table ─────────────────────────────────────────────────────────────────────

var (
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accent).
				PaddingRight(2)

	tableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)

	tableCellAltStyle = lipgloss.NewStyle().
				PaddingRight(2).
				Background(surfaceAlt)

	tableSelectedStyle = lipgloss.NewStyle().
				Background(surface).
				Foreground(highlight).
				Bold(true).
				PaddingRight(2)

	tableCursorStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	tableEmptyStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true).
			PaddingLeft(3)
)

// ─── Footer ────────────────────────────────────────────────────────────────────

var (
	footerKeyStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	footerDescStyle = lipgloss.NewStyle().
			Foreground(subtle)

	footerStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	footerSepStyle = lipgloss.NewStyle().
			Foreground(dimmed)
)

// ─── Filter ────────────────────────────────────────────────────────────────────

var (
	filterActiveStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	filterInactiveStyle = lipgloss.NewStyle().
				Foreground(subtle)

	filterIconStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)
)

// ─── Modal / Overlay ───────────────────────────────────────────────────────────

var (
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
			Foreground(lipgloss.Color("#CCCCCC"))

	errorTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(stoppedClr)
)

// ─── Info / Viewport ───────────────────────────────────────────────────────────

var (
	infoKeyStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	infoValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	infoBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2)
)

// ─── Spinner / Loading ─────────────────────────────────────────────────────────

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(accent)

	loadingMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			MarginLeft(1)
)

// ─── Form Styles ───────────────────────────────────────────────────────────────

var (
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
			Foreground(lipgloss.Color("#CCCCCC")).
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
)

// ─── Snapshot / Mount List ─────────────────────────────────────────────────────

var (
	listItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	listSelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Background(surface).
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
			Foreground(lipgloss.Color("#CCCCCC"))
)

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
