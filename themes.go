// themes.go - Theme definitions for the TUI
package main

import "github.com/charmbracelet/lipgloss"

// theme holds all configurable colors for the TUI.
type theme struct {
	Name       string
	Accent     lipgloss.Color
	AccentLight lipgloss.Color
	Subtle     lipgloss.Color
	Dimmed     lipgloss.Color
	Highlight  lipgloss.Color
	Surface    lipgloss.Color
	Text       lipgloss.Color // normal cell text
	TextMuted  lipgloss.Color // secondary text (values, descriptions)
	Running    lipgloss.Color
	Stopped    lipgloss.Color
	Suspended  lipgloss.Color
	Deleted    lipgloss.Color
}

// themes is the list of available themes, selectable via keys 1–0.
var themes = []theme{
	// 1 - Default Violet (inspired by VSCode default dark)
	{
		Name: "Violet", Accent: "#7C3AED", AccentLight: "#A78BFA",
		Subtle: "#6C6C6C", Dimmed: "#4A4A4A", Highlight: "#E8E8E8",
		Surface: "#2A2A2A", Text: "#BBBBBB", TextMuted: "#CCCCCC",
		Running: "#10B981", Stopped: "#EF4444", Suspended: "#F59E0B", Deleted: "#6B7280",
	},
	// 2 - Dracula
	{
		Name: "Dracula", Accent: "#BD93F9", AccentLight: "#D6BCFA",
		Subtle: "#6272A4", Dimmed: "#44475A", Highlight: "#F8F8F2",
		Surface: "#282A36", Text: "#BFBFBF", TextMuted: "#F8F8F2",
		Running: "#50FA7B", Stopped: "#FF5555", Suspended: "#FFB86C", Deleted: "#6272A4",
	},
	// 3 - Tokyo Night
	{
		Name: "Tokyo Night", Accent: "#7AA2F7", AccentLight: "#89DDFF",
		Subtle: "#565F89", Dimmed: "#3B4261", Highlight: "#C0CAF5",
		Surface: "#1A1B26", Text: "#A9B1D6", TextMuted: "#C0CAF5",
		Running: "#9ECE6A", Stopped: "#F7768E", Suspended: "#E0AF68", Deleted: "#565F89",
	},
	// 4 - One Dark (Atom)
	{
		Name: "One Dark", Accent: "#61AFEF", AccentLight: "#82CFFF",
		Subtle: "#5C6370", Dimmed: "#3E4451", Highlight: "#ABB2BF",
		Surface: "#282C34", Text: "#9DA5B4", TextMuted: "#ABB2BF",
		Running: "#98C379", Stopped: "#E06C75", Suspended: "#E5C07B", Deleted: "#5C6370",
	},
	// 5 - Monokai Pro
	{
		Name: "Monokai", Accent: "#A9DC76", AccentLight: "#C5E4A0",
		Subtle: "#727072", Dimmed: "#5B595C", Highlight: "#FCFCFA",
		Surface: "#2D2A2E", Text: "#BDBDBD", TextMuted: "#FCFCFA",
		Running: "#A9DC76", Stopped: "#FF6188", Suspended: "#FFD866", Deleted: "#727072",
	},
	// 6 - Nord
	{
		Name: "Nord", Accent: "#88C0D0", AccentLight: "#8FBCBB",
		Subtle: "#4C566A", Dimmed: "#3B4252", Highlight: "#ECEFF4",
		Surface: "#2E3440", Text: "#D8DEE9", TextMuted: "#ECEFF4",
		Running: "#A3BE8C", Stopped: "#BF616A", Suspended: "#EBCB8B", Deleted: "#4C566A",
	},
	// 7 - Catppuccin Mocha
	{
		Name: "Catppuccin", Accent: "#CBA6F7", AccentLight: "#F5C2E7",
		Subtle: "#6C7086", Dimmed: "#45475A", Highlight: "#CDD6F4",
		Surface: "#1E1E2E", Text: "#BAC2DE", TextMuted: "#CDD6F4",
		Running: "#A6E3A1", Stopped: "#F38BA8", Suspended: "#F9E2AF", Deleted: "#6C7086",
	},
	// 8 - Gruvbox Dark
	{
		Name: "Gruvbox", Accent: "#FE8019", AccentLight: "#FABD2F",
		Subtle: "#928374", Dimmed: "#665C54", Highlight: "#EBDBB2",
		Surface: "#282828", Text: "#BDAE93", TextMuted: "#EBDBB2",
		Running: "#B8BB26", Stopped: "#FB4934", Suspended: "#FABD2F", Deleted: "#928374",
	},
	// 9 - Solarized Dark
	{
		Name: "Solarized", Accent: "#268BD2", AccentLight: "#2AA198",
		Subtle: "#586E75", Dimmed: "#073642", Highlight: "#FDF6E3",
		Surface: "#002B36", Text: "#93A1A1", TextMuted: "#EEE8D5",
		Running: "#859900", Stopped: "#DC322F", Suspended: "#B58900", Deleted: "#586E75",
	},
	// 0 - Rosé Pine
	{
		Name: "Rosé Pine", Accent: "#C4A7E7", AccentLight: "#F6C177",
		Subtle: "#6E6A86", Dimmed: "#403D52", Highlight: "#E0DEF4",
		Surface: "#1F1D2E", Text: "#908CAA", TextMuted: "#E0DEF4",
		Running: "#9CCFD8", Stopped: "#EB6F92", Suspended: "#F6C177", Deleted: "#6E6A86",
	},
}

// currentThemeIndex tracks the active theme.
var currentThemeIndex = 8

// currentTheme returns the active theme.
func currentTheme() theme {
	return themes[currentThemeIndex]
}

// setTheme switches to the given theme index and rebuilds all styles.
func setTheme(index int) {
	if index < 0 || index >= len(themes) {
		return
	}
	currentThemeIndex = index
	rebuildStyles()
}
