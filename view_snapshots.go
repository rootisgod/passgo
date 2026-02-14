// view_snapshots.go - Snapshot creation form, manager list, and action views
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Snapshot Create Form ──────────────────────────────────────────────────────

type snapCreateModel struct {
	vmName      string
	nameInput   textinput.Model
	descInput   textinput.Model
	cursor      int // 0=name, 1=desc, 2=create, 3=cancel
	width       int
	height      int
}

func newSnapCreateModel(vmName string, w, h int) snapCreateModel {
	ni := textinput.New()
	ni.Placeholder = "snapshot-name"
	ni.CharLimit = 40
	ni.Focus()

	di := textinput.New()
	di.Placeholder = time.Now().Format("2006-01-02 15:04")
	di.SetValue(time.Now().Format("2006-01-02 15:04"))
	di.CharLimit = 80

	return snapCreateModel{
		vmName:    vmName,
		nameInput: ni,
		descInput: di,
		width:     w,
		height:    h,
	}
}

func (m snapCreateModel) Init() tea.Cmd { return textinput.Blink }

func (m snapCreateModel) Update(msg tea.Msg) (snapCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return backToTableMsg{} }
		case "tab", "down":
			m.blur()
			m.cursor = (m.cursor + 1) % 4
			m.focus()
			return m, nil
		case "shift+tab", "up":
			m.blur()
			m.cursor = (m.cursor - 1 + 4) % 4
			m.focus()
			return m, nil
		case "enter":
			if m.cursor == 3 { // cancel
				return m, func() tea.Msg { return backToTableMsg{} }
			}
			if m.cursor == 2 { // create
				name := strings.ReplaceAll(m.nameInput.Value(), " ", "-")
				if name == "" {
					return m, nil
				}
				desc := m.descInput.Value()
				return m, createSnapshotCmd(m.vmName, name, desc)
			}
			m.blur()
			m.cursor = (m.cursor + 1) % 4
			m.focus()
			return m, nil
		}

		// Forward to active input
		switch m.cursor {
		case 0:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		case 1:
			var cmd tea.Cmd
			m.descInput, cmd = m.descInput.Update(msg)
			return m, cmd
		}
	default:
		// Tick for cursor blink
		if m.cursor == 0 {
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
		if m.cursor == 1 {
			var cmd tea.Cmd
			m.descInput, cmd = m.descInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *snapCreateModel) blur() {
	m.nameInput.Blur()
	m.descInput.Blur()
}

func (m *snapCreateModel) focus() {
	switch m.cursor {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.descInput.Focus()
	}
}

func (m snapCreateModel) View() string {
	title := formTitleStyle.Render(fmt.Sprintf("Create Snapshot for: %s", m.vmName))

	nameLabel := formLabelStyle.Render("Name:")
	descLabel := formLabelStyle.Render("Description:")
	if m.cursor == 0 {
		nameLabel = formActiveLabelStyle.Render("Name:")
	}
	if m.cursor == 1 {
		descLabel = formActiveLabelStyle.Render("Description:")
	}

	var nameVal, descVal string
	if m.cursor == 0 {
		nameVal = m.nameInput.View()
	} else {
		nameVal = formValueStyle.Render(m.nameInput.Value())
	}
	if m.cursor == 1 {
		descVal = m.descInput.View()
	} else {
		descVal = formValueStyle.Render(m.descInput.Value())
	}

	createStyle := formButtonStyle
	cancelStyle := formButtonStyle
	if m.cursor == 2 {
		createStyle = formActiveButtonStyle
	}
	if m.cursor == 3 {
		cancelStyle = formActiveButtonStyle
	}

	content := title + "\n\n" +
		fmt.Sprintf("  %s  %s\n", lipgloss.NewStyle().Width(14).Render(nameLabel), nameVal) +
		fmt.Sprintf("  %s  %s\n\n", lipgloss.NewStyle().Width(14).Render(descLabel), descVal) +
		"  " + createStyle.Render("[ Create ]") + "  " + cancelStyle.Render("[ Cancel ]") + "\n\n" +
		formHintStyle.Render("Tab: navigate  Enter: submit  Esc: cancel")

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Snapshot Manager ──────────────────────────────────────────────────────────

type snapManageModel struct {
	vmName    string
	snapshots []SnapshotInfo
	cursor    int
	action    int // -1 = list, 0=revert, 1=delete, 2=cancel (when in actions mode)
	inActions bool
	width     int
	height    int
}

func newSnapManageModel(vmName string, w, h int) snapManageModel {
	return snapManageModel{vmName: vmName, cursor: 0, action: -1, width: w, height: h}
}

func (m snapManageModel) Update(msg tea.Msg) (snapManageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inActions {
			return m.updateActions(msg)
		}
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return backToTableMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.snapshots)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.snapshots) > 0 {
				m.inActions = true
				m.action = 0
			}
		}
	}
	return m, nil
}

func (m snapManageModel) updateActions(msg tea.KeyMsg) (snapManageModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inActions = false
		return m, nil
	case "left", "h":
		if m.action > 0 {
			m.action--
		}
	case "right", "l":
		if m.action < 2 {
			m.action++
		}
	case "enter":
		snap := m.snapshots[m.cursor]
		m.inActions = false
		switch m.action {
		case 0: // revert
			return m, restoreSnapshotCmd(m.vmName, snap.Name)
		case 1: // delete
			return m, deleteSnapshotCmd(m.vmName, snap.Name)
		case 2: // cancel
			return m, nil
		}
	}
	return m, nil
}

func (m snapManageModel) View() string {
	title := formTitleStyle.Render(fmt.Sprintf("Snapshots for: %s", m.vmName))

	if len(m.snapshots) == 0 {
		content := title + "\n\n" + tableEmptyStyle.Render("No snapshots found") + "\n\n" +
			formHintStyle.Render("Esc: return")
		box := modalStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	var lines []string
	for i, snap := range m.snapshots {
		selected := i == m.cursor
		text := snap.Name
		if snap.Comment != "" {
			text += " (" + snap.Comment + ")"
		}
		if snap.Parent != "" {
			text += " ← " + snap.Parent
		}

		if selected {
			lines = append(lines, listSelectedItemStyle.Render("▸ "+text))
		} else {
			lines = append(lines, listItemStyle.Render("  "+text))
		}
	}

	// Detail panel for selected
	var detail string
	if m.cursor >= 0 && m.cursor < len(m.snapshots) {
		s := m.snapshots[m.cursor]
		detail = detailKeyStyle.Render("Name: ") + detailValStyle.Render(s.Name) + "\n"
		if s.Comment != "" {
			detail += detailKeyStyle.Render("Comment: ") + detailValStyle.Render(s.Comment) + "\n"
		}
		if s.Parent != "" {
			detail += detailKeyStyle.Render("Parent: ") + detailValStyle.Render(s.Parent) + "\n"
		}
	}

	// Actions overlay
	var actionsLine string
	if m.inActions {
		actions := []string{"Revert", "Delete", "Cancel"}
		var buttons []string
		for i, a := range actions {
			style := formButtonStyle
			if i == m.action {
				style = formActiveButtonStyle
			}
			buttons = append(buttons, style.Render(" "+a+" "))
		}
		actionsLine = "\n" + strings.Join(buttons, "  ")
	}

	hint := formHintStyle.Render("↑↓: navigate  Enter: actions  Esc: return")

	content := title + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		detailPanelStyle.Render(detail) +
		actionsLine + "\n\n" + hint

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
