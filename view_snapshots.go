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

// snapTreeNode represents a snapshot in a tree structure.
type snapTreeNode struct {
	snap     SnapshotInfo
	children []*snapTreeNode
}

type snapManageModel struct {
	vmName    string
	snapshots []SnapshotInfo
	tree      []snapTreeEntry // snapshots in tree display order
	cursor    int
	action    int // -1 = list, 0=revert, 1=delete, 2=cancel (when in actions mode)
	inActions bool
	width     int
	height    int
}

func newSnapManageModel(vmName string, w, h int) snapManageModel {
	return snapManageModel{vmName: vmName, cursor: 0, action: -1, width: w, height: h}
}

// setSnapshots stores the snapshots and pre-computes the tree order.
func (m *snapManageModel) setSnapshots(snaps []SnapshotInfo) {
	m.snapshots = snaps
	m.tree = buildSnapTree(snaps)
}

// snapTreeEntry is a flattened tree row with its display prefix and depth.
type snapTreeEntry struct {
	snap   SnapshotInfo
	prefix string // tree-drawing characters (e.g. "├── ", "│   └── ")
	depth  int
}

// buildSnapTree builds a tree from the flat snapshot list and returns it
// as a flattened display order with tree-drawing prefixes.
func buildSnapTree(snapshots []SnapshotInfo) []snapTreeEntry {
	if len(snapshots) == 0 {
		return nil
	}

	// Build lookup: name -> snapshot
	byName := make(map[string]*snapTreeNode, len(snapshots))
	for _, s := range snapshots {
		byName[s.Name] = &snapTreeNode{snap: s}
	}

	// Build tree: attach children to parents
	var roots []*snapTreeNode
	for _, s := range snapshots {
		node := byName[s.Name]
		if s.Parent != "" {
			if parent, ok := byName[s.Parent]; ok {
				parent.children = append(parent.children, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	// Flatten tree with YAML-style dash prefixes using DFS
	var entries []snapTreeEntry
	var walk func(nodes []*snapTreeNode, depth int)
	walk = func(nodes []*snapTreeNode, depth int) {
		for _, node := range nodes {
			prefix := ""
			if depth > 0 {
				prefix = strings.Repeat("  ", depth-1) + "- "
			}
			entries = append(entries, snapTreeEntry{
				snap:   node.snap,
				prefix: prefix,
				depth:  depth,
			})
			walk(node.children, depth+1)
		}
	}
	walk(roots, 0)
	return entries
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
			if m.cursor < len(m.tree)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.tree) > 0 {
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
		snap := m.tree[m.cursor].snap
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
	title := modalTitleStyle.Render(fmt.Sprintf("Snapshots for: %s", m.vmName))

	if len(m.tree) == 0 {
		content := title + "\n\n" + tableEmptyStyle.Render("No snapshots found") + "\n\n" +
			formHintStyle.Render("Esc: return")
		box := modalStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	tree := m.tree

	// Available content width inside modal
	modalW := min(80, m.width-4)
	avail := modalW - 6 - 1 // padding(6) + cursor prefix(1)

	// ── Header row ──
	headerDiv := tableHeaderDivStyle.Render("│")

	// Find the widest tree entry (prefix + name) to size the name column
	maxNameW := len("Snapshot")
	for _, e := range tree {
		w := len(e.prefix) + len(e.snap.Name)
		if w > maxNameW {
			maxNameW = w
		}
	}
	nameColW := min(maxNameW+2, avail*2/3)
	if nameColW < 12 {
		nameColW = 12
	}
	commentColW := avail - nameColW - 1 // -1 for divider
	showComment := commentColW >= 8

	var headerRow string
	if showComment {
		headerRow = " " + tableHeaderStyle.Width(nameColW).Render("Snapshot") +
			headerDiv + tableHeaderStyle.Width(commentColW).Render("Comment")
	} else {
		headerRow = " " + tableHeaderStyle.Width(avail).Render("Snapshot")
		nameColW = avail
	}

	// ── Separator row ──
	dashStyle := lipgloss.NewStyle().Foreground(dimmed)
	var sepRow string
	if showComment {
		sepRow = " " + dashStyle.Render(strings.Repeat("─", nameColW)) +
			tableColDivStyle.Render("┼") +
			dashStyle.Render(strings.Repeat("─", commentColW))
	} else {
		sepRow = " " + dashStyle.Render(strings.Repeat("─", avail))
	}

	// ── Tree rows ──
	div := tableColDivStyle.Render("│")

	var rows []string
	for i, entry := range tree {
		selected := i == m.cursor

		cursor := " "
		if selected {
			cursor = tableCursorStyle.Render("▎")
		}

		cellStyle := func(width int) lipgloss.Style {
			if selected {
				return tableSelectedCellStyle.Width(width)
			}
			return tableCellStyle.Width(width)
		}

		// Build name cell: tree prefix + snapshot name (same color)
		treePfx := entry.prefix
		name := entry.snap.Name
		// Truncate name if needed (accounting for prefix visual width)
		prefixW := lipgloss.Width(treePfx)
		maxName := nameColW - prefixW - 1
		if maxName < 0 {
			maxName = 0
		}
		if len(name) > maxName && maxName > 3 {
			name = name[:maxName-1] + "…"
		}
		nameContent := treePfx + name
		// Pad to column width
		nameVisW := lipgloss.Width(nameContent)
		if nameVisW < nameColW {
			nameContent += strings.Repeat(" ", nameColW-nameVisW)
		}
		// Apply selection background
		if selected {
			nameContent = tableSelectedCellStyle.Render(nameContent)
		}

		var row string
		if showComment {
			comment := entry.snap.Comment
			if len(comment) > commentColW-1 && commentColW > 4 {
				comment = comment[:commentColW-3] + "…"
			}
			row = cursor + nameContent + div + cellStyle(commentColW).Render(comment)
		} else {
			row = cursor + nameContent
		}
		rows = append(rows, row)
	}

	tableContent := headerRow + "\n" + sepRow + "\n" + strings.Join(rows, "\n")

	// ── Actions overlay ──
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
		actionsLine = "\n\n " + strings.Join(buttons, "  ")
	}

	// ── Footer hints ──
	hint := footerKeyStyle.Render("↑↓") + " " + footerDescStyle.Render("navigate") + "  " +
		footerKeyStyle.Render("Enter") + " " + footerDescStyle.Render("actions") + "  " +
		footerKeyStyle.Render("Esc") + " " + footerDescStyle.Render("return")

	content := title + "\n" +
		tableContent +
		actionsLine + "\n\n" + hint

	box := modalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
