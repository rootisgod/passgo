// view_mounts.go - Mount manager, add mount, and modify mount views
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Mount Manager ─────────────────────────────────────────────────────────────

type mountManageModel struct {
	vmName string
	mounts []MountInfo
	cursor int
	action int // -1=list, 0=modify, 1=delete, 2=cancel
	inActions bool
	width  int
	height int
}

func newMountManageModel(vmName string, w, h int) mountManageModel {
	return mountManageModel{vmName: vmName, width: w, height: h}
}

// mountAddRequestMsg asks root to switch to mount add view.
type mountAddRequestMsg struct{ vmName string }

// mountModifyRequestMsg asks root to switch to mount modify view.
type mountModifyRequestMsg struct {
	vmName string
	mount  MountInfo
}

func (m mountManageModel) Update(msg tea.Msg) (mountManageModel, tea.Cmd) {
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
			if m.cursor < len(m.mounts)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.mounts) > 0 {
				m.inActions = true
				m.action = 0
			}
		case "a":
			return m, func() tea.Msg { return mountAddRequestMsg{vmName: m.vmName} }
		case "d":
			if len(m.mounts) > 0 && m.cursor < len(m.mounts) {
				mount := m.mounts[m.cursor]
				return m, umountCmd(m.vmName, mount.TargetPath)
			}
		case "e":
			if len(m.mounts) > 0 && m.cursor < len(m.mounts) {
				mount := m.mounts[m.cursor]
				return m, func() tea.Msg {
					return mountModifyRequestMsg{vmName: m.vmName, mount: mount}
				}
			}
		}
	}
	return m, nil
}

func (m mountManageModel) updateActions(msg tea.KeyMsg) (mountManageModel, tea.Cmd) {
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
		mount := m.mounts[m.cursor]
		m.inActions = false
		switch m.action {
		case 0: // modify
			return m, func() tea.Msg {
				return mountModifyRequestMsg{vmName: m.vmName, mount: mount}
			}
		case 1: // delete
			return m, umountCmd(m.vmName, mount.TargetPath)
		case 2: // cancel
			return m, nil
		}
	}
	return m, nil
}

func (m mountManageModel) View() string {
	title := formTitleStyle.Render(fmt.Sprintf("Mounts for: %s (%d)", m.vmName, len(m.mounts)))

	if len(m.mounts) == 0 {
		content := title + "\n\n" +
			tableEmptyStyle.Render("No mounts configured") + "\n\n" +
			formHintStyle.Render("a: add mount  Esc: return")
		box := modalStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	// Table header
	srcW := 36
	tgtW := 36
	header := tableHeaderStyle.Width(srcW).Render("Source (Local)") +
		tableHeaderStyle.Width(4).Render(" → ") +
		tableHeaderStyle.Width(tgtW).Render("Target (VM)")

	var rows []string
	for i, mount := range m.mounts {
		selected := i == m.cursor
		src := mount.SourcePath
		tgt := mount.TargetPath
		if len(src) > srcW-2 {
			src = "…" + src[len(src)-srcW+3:]
		}
		if len(tgt) > tgtW-2 {
			tgt = "…" + tgt[len(tgt)-tgtW+3:]
		}

		style := tableCellStyle
		if selected {
			style = tableSelectedCellStyle
		}

		row := style.Width(srcW).Render(src) +
			style.Width(4).Render(" → ") +
			style.Width(tgtW).Render(tgt)

		prefix := "  "
		if selected {
			prefix = tableCursorStyle.Render("▎ ")
		}
		rows = append(rows, prefix+row)
	}

	// Detail for selected
	var detail string
	if m.cursor >= 0 && m.cursor < len(m.mounts) {
		mount := m.mounts[m.cursor]
		detail = detailKeyStyle.Render("Source: ") + detailValStyle.Render(mount.SourcePath) + "\n" +
			detailKeyStyle.Render("Target: ") + detailValStyle.Render(mount.TargetPath)
	}

	// Actions
	var actionsLine string
	if m.inActions {
		actions := []string{"Modify", "Delete", "Cancel"}
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

	hint := formHintStyle.Render("a: add  e: modify  d: delete  Enter: actions  Esc: return")

	content := title + "\n\n" + header + "\n" +
		strings.Join(rows, "\n") + "\n\n" +
		detailPanelStyle.Render(detail) +
		actionsLine + "\n\n" + hint

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Mount Add (File picker + Target form) ─────────────────────────────────────

type mountAddModel struct {
	vmName      string
	phase       int // 0=browse, 1=target form
	// File browser
	currentDir  string
	entries     []os.DirEntry
	dirCursor   int
	dirOffset   int
	showHidden  bool
	// Target form
	sourceInput textinput.Model
	targetInput textinput.Model
	formCursor  int // 0=source, 1=target, 2=mount, 3=cancel
	width       int
	height      int
}

func newMountAddModel(vmName string, w, h int) mountAddModel {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/"
	}

	si := textinput.New()
	si.CharLimit = 200

	ti := textinput.New()
	ti.CharLimit = 200

	m := mountAddModel{
		vmName:      vmName,
		phase:       0,
		currentDir:  homeDir,
		sourceInput: si,
		targetInput: ti,
		width:       w,
		height:      h,
	}
	m.loadDir()
	return m
}

func (m *mountAddModel) loadDir() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.entries = nil
		return
	}
	var dirs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !m.showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirs = append(dirs, e)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name()) < strings.ToLower(dirs[j].Name())
	})
	m.entries = dirs
	m.dirCursor = 0
	m.dirOffset = 0
}

func (m mountAddModel) Init() tea.Cmd { return nil }

func (m mountAddModel) Update(msg tea.Msg) (mountAddModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.phase == 0 {
			return m.updateBrowser(msg)
		}
		return m.updateTargetForm(msg)
	default:
		// Blink for target form
		if m.phase == 1 {
			switch m.formCursor {
			case 0:
				var cmd tea.Cmd
				m.sourceInput, cmd = m.sourceInput.Update(msg)
				return m, cmd
			case 1:
				var cmd tea.Cmd
				m.targetInput, cmd = m.targetInput.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m mountAddModel) updateBrowser(msg tea.KeyMsg) (mountAddModel, tea.Cmd) {
	visible := max(1, m.height-8)
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return backToTableMsg{} }
	case "up", "k":
		if m.dirCursor > 0 {
			m.dirCursor--
			if m.dirCursor < m.dirOffset {
				m.dirOffset = m.dirCursor
			}
		}
	case "down", "j":
		if m.dirCursor < len(m.entries)-1 {
			m.dirCursor++
			if m.dirCursor >= m.dirOffset+visible {
				m.dirOffset = m.dirCursor - visible + 1
			}
		}
	case "enter":
		// Select the highlighted directory as source
		selectedDir := m.currentDir
		if len(m.entries) > 0 && m.dirCursor < len(m.entries) {
			selectedDir = filepath.Join(m.currentDir, m.entries[m.dirCursor].Name())
		}
		m.phase = 1
		m.sourceInput.SetValue(selectedDir)
		baseName := filepath.Base(selectedDir)
		m.targetInput.SetValue("/home/ubuntu/" + baseName)
		m.targetInput.Focus()
		m.formCursor = 1
		return m, textinput.Blink
	case " ":
		// Enter selected subdirectory
		if len(m.entries) > 0 && m.dirCursor < len(m.entries) {
			entry := m.entries[m.dirCursor]
			m.currentDir = filepath.Join(m.currentDir, entry.Name())
			m.loadDir()
		}
	case "backspace", "u":
		// Go up
		parent := filepath.Dir(m.currentDir)
		if parent != m.currentDir {
			m.currentDir = parent
			m.loadDir()
		}
	case ".":
		m.showHidden = !m.showHidden
		m.loadDir()
	}
	return m, nil
}

func (m mountAddModel) updateTargetForm(msg tea.KeyMsg) (mountAddModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.phase = 0
		return m, nil
	case "tab", "down":
		m.blurForm()
		m.formCursor = (m.formCursor + 1) % 4
		m.focusForm()
		return m, nil
	case "shift+tab", "up":
		m.blurForm()
		m.formCursor = (m.formCursor - 1 + 4) % 4
		m.focusForm()
		return m, nil
	case "enter":
		if m.formCursor == 3 { // cancel
			m.phase = 0
			return m, nil
		}
		if m.formCursor == 2 { // mount
			source := m.sourceInput.Value()
			target := m.targetInput.Value()
			if source == "" || target == "" {
				return m, nil
			}
			return m, mountCmd(source, m.vmName, target)
		}
		m.blurForm()
		m.formCursor = (m.formCursor + 1) % 4
		m.focusForm()
		return m, nil
	}

	switch m.formCursor {
	case 0:
		var cmd tea.Cmd
		m.sourceInput, cmd = m.sourceInput.Update(msg)
		return m, cmd
	case 1:
		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *mountAddModel) blurForm() {
	m.sourceInput.Blur()
	m.targetInput.Blur()
}

func (m *mountAddModel) focusForm() {
	switch m.formCursor {
	case 0:
		m.sourceInput.Focus()
	case 1:
		m.targetInput.Focus()
	}
}

func (m mountAddModel) View() string {
	if m.phase == 0 {
		return m.viewBrowser()
	}
	return m.viewTargetForm()
}

func (m mountAddModel) viewBrowser() string {
	title := formTitleStyle.Render("Browse Directories")
	pathLine := detailKeyStyle.Render("Path: ") + detailValStyle.Render(m.currentDir)

	visible := max(1, m.height-8)
	var lines []string
	if len(m.entries) == 0 {
		lines = append(lines, tableEmptyStyle.Render("(empty directory)"))
	} else {
		end := min(m.dirOffset+visible, len(m.entries))
		for i := m.dirOffset; i < end; i++ {
			entry := m.entries[i]
			selected := i == m.dirCursor
			name := entry.Name() + "/"
			if selected {
				lines = append(lines, listSelectedItemStyle.Render("▸ "+name))
			} else {
				lines = append(lines, listItemStyle.Render("  "+name))
			}
		}
	}

	// Hidden indicator
	hiddenStr := ""
	if m.showHidden {
		hiddenStr = " (showing hidden)"
	}

	hint := formHintStyle.Render("↑↓: navigate  Space: enter dir  Enter: select  u: up  .: toggle hidden  Esc: cancel" + hiddenStr)

	content := title + "\n" + pathLine + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" + hint

	box := modalStyle.MaxWidth(m.width - 4).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m mountAddModel) viewTargetForm() string {
	title := formTitleStyle.Render(fmt.Sprintf("Add Mount to: %s", m.vmName))

	srcLabel := formLabelStyle.Render("Source (Local):")
	tgtLabel := formLabelStyle.Render("Target (VM):")
	if m.formCursor == 0 {
		srcLabel = formActiveLabelStyle.Render("Source (Local):")
	}
	if m.formCursor == 1 {
		tgtLabel = formActiveLabelStyle.Render("Target (VM):")
	}

	var srcVal, tgtVal string
	if m.formCursor == 0 {
		srcVal = m.sourceInput.View()
	} else {
		srcVal = formValueStyle.Render(m.sourceInput.Value())
	}
	if m.formCursor == 1 {
		tgtVal = m.targetInput.View()
	} else {
		tgtVal = formValueStyle.Render(m.targetInput.Value())
	}

	mountStyle := formButtonStyle
	cancelStyle := formButtonStyle
	if m.formCursor == 2 {
		mountStyle = formActiveButtonStyle
	}
	if m.formCursor == 3 {
		cancelStyle = formActiveButtonStyle
	}

	hint := formHintStyle.Render("Tab: navigate  Enter: submit  Esc: back to browser")

	content := title + "\n\n" +
		fmt.Sprintf("  %s  %s\n", lipgloss.NewStyle().Width(16).Render(srcLabel), srcVal) +
		fmt.Sprintf("  %s  %s\n\n", lipgloss.NewStyle().Width(16).Render(tgtLabel), tgtVal) +
		"  " + mountStyle.Render("[ Mount ]") + "  " + cancelStyle.Render("[ Cancel ]") + "\n\n" + hint

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Mount Modify ──────────────────────────────────────────────────────────────

type mountModifyModel struct {
	vmName       string
	oldMount     MountInfo
	sourceInput  textinput.Model
	targetInput  textinput.Model
	cursor       int // 0=source, 1=target, 2=save, 3=cancel
	width        int
	height       int
}

func newMountModifyModel(vmName string, mount MountInfo, w, h int) mountModifyModel {
	si := textinput.New()
	si.SetValue(mount.SourcePath)
	si.CharLimit = 200
	si.Focus()

	ti := textinput.New()
	ti.SetValue(mount.TargetPath)
	ti.CharLimit = 200

	return mountModifyModel{
		vmName:      vmName,
		oldMount:    mount,
		sourceInput: si,
		targetInput: ti,
		width:       w,
		height:      h,
	}
}

// mountModifySubmitMsg asks root to unmount old and mount new.
type mountModifySubmitMsg struct {
	vmName    string
	oldTarget string
	newSource string
	newTarget string
}

func (m mountModifyModel) Init() tea.Cmd { return textinput.Blink }

func (m mountModifyModel) Update(msg tea.Msg) (mountModifyModel, tea.Cmd) {
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
			if m.cursor == 3 {
				return m, func() tea.Msg { return backToTableMsg{} }
			}
			if m.cursor == 2 {
				src := m.sourceInput.Value()
				tgt := m.targetInput.Value()
				if src == "" || tgt == "" {
					return m, nil
				}
				return m, func() tea.Msg {
					return mountModifySubmitMsg{
						vmName:    m.vmName,
						oldTarget: m.oldMount.TargetPath,
						newSource: src,
						newTarget: tgt,
					}
				}
			}
			m.blur()
			m.cursor = (m.cursor + 1) % 4
			m.focus()
			return m, nil
		}

		switch m.cursor {
		case 0:
			var cmd tea.Cmd
			m.sourceInput, cmd = m.sourceInput.Update(msg)
			return m, cmd
		case 1:
			var cmd tea.Cmd
			m.targetInput, cmd = m.targetInput.Update(msg)
			return m, cmd
		}
	default:
		if m.cursor == 0 {
			var cmd tea.Cmd
			m.sourceInput, cmd = m.sourceInput.Update(msg)
			return m, cmd
		}
		if m.cursor == 1 {
			var cmd tea.Cmd
			m.targetInput, cmd = m.targetInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *mountModifyModel) blur() {
	m.sourceInput.Blur()
	m.targetInput.Blur()
}

func (m *mountModifyModel) focus() {
	switch m.cursor {
	case 0:
		m.sourceInput.Focus()
	case 1:
		m.targetInput.Focus()
	}
}

func (m mountModifyModel) View() string {
	title := formTitleStyle.Render(fmt.Sprintf("Modify Mount for: %s", m.vmName))

	srcLabel := formLabelStyle.Render("Source (Local):")
	tgtLabel := formLabelStyle.Render("Target (VM):")
	if m.cursor == 0 {
		srcLabel = formActiveLabelStyle.Render("Source (Local):")
	}
	if m.cursor == 1 {
		tgtLabel = formActiveLabelStyle.Render("Target (VM):")
	}

	var srcVal, tgtVal string
	if m.cursor == 0 {
		srcVal = m.sourceInput.View()
	} else {
		srcVal = formValueStyle.Render(m.sourceInput.Value())
	}
	if m.cursor == 1 {
		tgtVal = m.targetInput.View()
	} else {
		tgtVal = formValueStyle.Render(m.targetInput.Value())
	}

	saveStyle := formButtonStyle
	cancelStyle := formButtonStyle
	if m.cursor == 2 {
		saveStyle = formActiveButtonStyle
	}
	if m.cursor == 3 {
		cancelStyle = formActiveButtonStyle
	}

	hint := formHintStyle.Render("Tab: navigate  Enter: submit  Esc: cancel")

	content := title + "\n\n" +
		fmt.Sprintf("  %s  %s\n", lipgloss.NewStyle().Width(16).Render(srcLabel), srcVal) +
		fmt.Sprintf("  %s  %s\n\n", lipgloss.NewStyle().Width(16).Render(tgtLabel), tgtVal) +
		"  " + saveStyle.Render("[ Save ]") + "  " + cancelStyle.Render("[ Cancel ]") + "\n\n" + hint

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
