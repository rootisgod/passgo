// view_modals.go - Help, version, error, and confirm modal views
package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Help Modal ────────────────────────────────────────────────────────────────

type helpModel struct {
	width  int
	height int
}

func newHelpModel() helpModel {
	return helpModel{}
}

func (m helpModel) View() string {
	title := modalTitleStyle.Render("Keyboard Shortcuts")

	shortcuts := []struct{ key, desc string }{
		{"h", "Help"},
		{"i", "VM Info"},
		{"c", "Quick Create"},
		{"C", "Advanced Create (cloud-init)"},
		{"[", "Stop selected VM"},
		{"]", "Start selected VM"},
		{"p", "Suspend selected VM"},
		{"<", "Stop ALL VMs"},
		{">", "Start ALL VMs"},
		{"d", "Delete selected VM"},
		{"r", "Recover deleted VM"},
		{"!", "Purge ALL deleted VMs"},
		{"/", "Refresh VM list"},
		{"f", "Filter VMs by name"},
		{"s", "Shell (interactive session)"},
		{"n", "Create snapshot"},
		{"m", "Manage snapshots"},
		{"M", "Manage mounts"},
		{"o", "VM Options (CPU/RAM/Disk)"},
		{"v", "Version"},
		{"1-0", "Switch theme (1-9, 0)"},
		{"q", "Quit"},
	}

	var lines []string
	for _, s := range shortcuts {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			footerKeyStyle.Width(4).Render(s.key),
			modalTextStyle.Render(s.desc)))
	}

	// Theme list
	themeLines := []string{"", formActiveLabelStyle.Render("  Themes:")}
	for i, t := range themes {
		key := fmt.Sprintf("%d", i+1)
		if i == 9 {
			key = "0"
		}
		marker := "  "
		if i == currentThemeIndex {
			marker = "● "
		}
		swatch := lipgloss.NewStyle().Foreground(t.Accent).Render("██")
		themeLines = append(themeLines, fmt.Sprintf("  %s%s %s %s",
			marker,
			footerKeyStyle.Width(2).Render(key),
			swatch,
			modalTextStyle.Render(t.Name)))
	}

	hints := []string{
		"",
		formHintStyle.Render("Tab/Shift+Tab: cycle/toggle sort column"),
		formHintStyle.Render("Filter: press f, type name, Esc/Enter to close"),
		formHintStyle.Render("Shell: suspends TUI, restores on exit"),
		"",
		formHintStyle.Render("Press Esc or Enter to close"),
	}

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n" + strings.Join(themeLines, "\n") + "\n" + strings.Join(hints, "\n")
	box := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Version Modal ─────────────────────────────────────────────────────────────

type versionModel struct {
	width  int
	height int
}

func newVersionModel() versionModel {
	return versionModel{}
}

func (m versionModel) View() string {
	title := modalTitleStyle.Render("Version")
	body := modalTextStyle.Render(GetVersion())
	hint := "\n\n" + formHintStyle.Render("Press Esc or Enter to close")

	content := title + "\n\n" + body + hint
	box := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Error Modal ───────────────────────────────────────────────────────────────

type errorModel struct {
	title   string
	message string
	width   int
	height  int
}

func newErrorModel(title, message string) errorModel {
	return errorModel{title: title, message: message}
}

func (m errorModel) View() string {
	t := errorTitleStyle.Render(m.title)
	body := modalTextStyle.Render(m.message)
	hint := "\n\n" + formHintStyle.Render("Press Esc or Enter to close")

	content := t + "\n\n" + body + hint
	box := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Confirm Modal ─────────────────────────────────────────────────────────────

type confirmModel struct {
	question string
	cursor   int // 0=Yes, 1=No
	width    int
	height   int
}

func newConfirmModel(question string) confirmModel {
	return confirmModel{question: question}
}

func (m confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.cursor = 0
		case "right", "l":
			m.cursor = 1
		case "y", "Y":
			return m, func() tea.Msg { return confirmResultMsg{confirmed: true} }
		case "n", "N", "esc":
			return m, func() tea.Msg { return confirmResultMsg{confirmed: false} }
		case "enter":
			confirmed := m.cursor == 0
			return m, func() tea.Msg { return confirmResultMsg{confirmed: confirmed} }
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	title := modalTitleStyle.Render("Confirm")
	body := modalTextStyle.Render(m.question)

	yesStyle := formButtonStyle
	noStyle := formButtonStyle
	if m.cursor == 0 {
		yesStyle = formActiveButtonStyle
	} else {
		noStyle = formActiveButtonStyle
	}

	buttons := yesStyle.Render(" Yes ") + "  " + noStyle.Render(" No ")
	hint := formHintStyle.Render("y/n or ←→ + Enter")

	content := title + "\n\n" + body + "\n\n" + buttons + "\n\n" + hint
	box := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
