// view_info.go - Scrollable VM details view using viewport
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type infoModel struct {
	vmName   string
	viewport viewport.Model
	content  string
	ready    bool
	width    int
	height   int
}

func newInfoModel(vmName string, width, height int) infoModel {
	return infoModel{vmName: vmName, width: width, height: height}
}

func (m *infoModel) setContent(raw string) {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			b.WriteString(infoKeyStyle.Render(parts[0]+":") + infoValStyle.Render(parts[1]) + "\n")
		} else if trimmed != "" {
			b.WriteString(infoValStyle.Render(line) + "\n")
		} else {
			b.WriteString("\n")
		}
	}
	m.content = b.String()

	vpWidth := min(m.width-6, 76)
	vpHeight := m.height - 6

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
	m.viewport.SetContent(m.content)
}

func (m infoModel) Update(msg tea.Msg) (infoModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter", "q":
			return m, func() tea.Msg { return backToTableMsg{} }
		}
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m infoModel) View() string {
	title := modalTitleStyle.Render(fmt.Sprintf("Info: %s", m.vmName))

	var body string
	if !m.ready {
		body = loadingMsgStyle.Render("Loading…")
	} else {
		body = m.viewport.View()
	}

	hint := formHintStyle.Render("↑↓ scroll  Esc close")
	content := title + "\n\n" + body + "\n\n" + hint

	box := infoBorderStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
