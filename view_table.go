// view_table.go - Main VM table with filter bar, sorting, and footer
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Column definitions for the VM table.
type tableColumn struct {
	title string
	width int
}

type tableModel struct {
	vms         []vmData
	filteredVMs []vmData
	cursor      int
	offset      int
	width       int
	height      int

	filterInput   textinput.Model
	filterFocused bool
	filterVisible bool
	filterText    string

	sortColumn    int
	sortAscending bool

	columns []tableColumn
}

func newTableModel() tableModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Prompt = "Filter: "
	ti.PromptStyle = filterActiveStyle
	ti.CharLimit = 64

	return tableModel{
		filterInput:   ti,
		sortColumn:    0,
		sortAscending: true,
		columns: []tableColumn{
			{title: "Name", width: 18},
			{title: "State", width: 12},
			{title: "Snaps", width: 7},
			{title: "IPv4", width: 16},
			{title: "Release", width: 22},
			{title: "CPUs", width: 6},
			{title: "Disk", width: 18},
			{title: "Memory", width: 18},
			{title: "Mounts", width: 10},
		},
	}
}

// ─── Data ──────────────────────────────────────────────────────────────────────

func (m *tableModel) setVMs(vms []vmData) {
	m.vms = vms
	m.applyFilterAndSort()
	if m.cursor >= len(m.filteredVMs) {
		m.cursor = max(0, len(m.filteredVMs)-1)
	}
}

func (m *tableModel) applyFilterAndSort() {
	filter := strings.ToLower(m.filterText)
	m.filteredVMs = nil
	for _, vm := range m.vms {
		if filter == "" || strings.Contains(strings.ToLower(vm.info.Name), filter) {
			m.filteredVMs = append(m.filteredVMs, vm)
		}
	}
	sortVMs(m.filteredVMs, m.sortColumn, m.sortAscending)
}

func (m *tableModel) selectedVM() (VMInfo, bool) {
	if m.cursor >= 0 && m.cursor < len(m.filteredVMs) {
		return m.filteredVMs[m.cursor].info, true
	}
	return VMInfo{}, false
}

func (m *tableModel) allVMNames() []string {
	var names []string
	for _, vm := range m.vms {
		names = append(names, vm.info.Name)
	}
	return names
}

func (m *tableModel) toggleFilter() {
	if m.filterVisible && m.filterFocused {
		m.filterFocused = false
		if m.filterText == "" {
			m.filterVisible = false
		}
		return
	}
	if m.filterVisible && !m.filterFocused {
		m.filterFocused = true
		m.filterInput.Focus()
		return
	}
	m.filterVisible = true
	m.filterFocused = true
	m.filterInput.Focus()
}

func (m *tableModel) cycleSortColumn() {
	m.sortColumn = (m.sortColumn + 1) % len(m.columns)
	m.applyFilterAndSort()
}

func (m *tableModel) toggleSortDirection() {
	m.sortAscending = !m.sortAscending
	m.applyFilterAndSort()
}

// ─── Update ────────────────────────────────────────────────────────────────────

func (m tableModel) Update(msg tea.Msg) (tableModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filterFocused {
			switch msg.String() {
			case "esc":
				m.filterFocused = false
				if m.filterText == "" {
					m.filterVisible = false
				}
				return m, nil
			case "enter":
				m.filterFocused = false
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filterText = m.filterInput.Value()
				m.applyFilterAndSort()
				if m.cursor >= len(m.filteredVMs) {
					m.cursor = max(0, len(m.filteredVMs)-1)
				}
				return m, cmd
			}
		}

		visible := m.visibleRows()
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.filteredVMs)-1 {
				m.cursor++
				if m.cursor >= m.offset+visible {
					m.offset = m.cursor - visible + 1
				}
			}
		case "home", "g":
			m.cursor = 0
			m.offset = 0
		case "end", "G":
			m.cursor = max(0, len(m.filteredVMs)-1)
			if m.cursor >= visible {
				m.offset = m.cursor - visible + 1
			}
		case "pgup":
			m.cursor = max(0, m.cursor-visible)
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		case "pgdown":
			m.cursor = min(len(m.filteredVMs)-1, m.cursor+visible)
			if m.cursor >= m.offset+visible {
				m.offset = m.cursor - visible + 1
			}
		case "tab":
			m.cycleSortColumn()
		case "shift+tab":
			m.toggleSortDirection()
		}
	}
	return m, nil
}

func (m tableModel) visibleRows() int {
	// title(1) + filter(0/1) + header(1) + sep(1) + footer(3) + spacing(1)
	used := 7
	if m.filterVisible {
		used++
	}
	return max(1, m.height-used)
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m tableModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("╭ Multipass VMs ╮") + "\n")

	// Filter bar
	if m.filterVisible {
		if m.filterFocused {
			b.WriteString(m.filterInput.View() + "\n")
		} else {
			dim := filterInactiveStyle.Render(fmt.Sprintf("  Filter: %s", m.filterText))
			b.WriteString(dim + "\n")
		}
	}

	// Compute dynamic column widths
	cols := m.computeColumnWidths()

	// Header row
	var headerCells []string
	for i, col := range cols {
		title := col.title
		if i == m.sortColumn {
			if m.sortAscending {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}
		headerCells = append(headerCells, tableHeaderStyle.Width(col.width).Render(title))
	}
	b.WriteString("  " + strings.Join(headerCells, "") + "\n")

	// Separator
	sepLen := 0
	for _, c := range cols {
		sepLen += c.width
	}
	sep := lipgloss.NewStyle().Foreground(subtle).Render(strings.Repeat("─", min(sepLen+2, m.width)))
	b.WriteString("  " + sep + "\n")

	// Rows
	visible := m.visibleRows()
	if len(m.filteredVMs) == 0 {
		b.WriteString(tableEmptyStyle.Render("No VMs found") + "\n")
	} else {
		end := min(m.offset+visible, len(m.filteredVMs))
		for i := m.offset; i < end; i++ {
			vm := m.filteredVMs[i]
			selected := i == m.cursor
			b.WriteString(m.renderRow(vm, cols, selected) + "\n")
		}
	}

	// Pad remaining space
	rendered := len(m.filteredVMs) - m.offset
	if rendered < 0 {
		rendered = 0
	}
	if rendered > visible {
		rendered = visible
	}
	for i := rendered; i < visible; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m tableModel) computeColumnWidths() []tableColumn {
	cols := make([]tableColumn, len(m.columns))
	copy(cols, m.columns)

	// Compute total minimum width
	total := 0
	for _, c := range cols {
		total += c.width
	}
	total += 4 // cursor prefix + padding

	// If terminal is wider, expand Name and Release columns
	extra := m.width - total
	if extra > 0 {
		cols[0].width += min(extra/2, 12) // Name
		extra -= min(extra/2, 12)
		if extra > 0 {
			cols[4].width += min(extra, 10) // Release
		}
	}

	return cols
}

func (m tableModel) renderRow(vm vmData, cols []tableColumn, selected bool) string {
	values := []string{
		vm.info.Name,
		vm.info.State,
		vm.info.Snapshots,
		vm.info.IPv4,
		vm.info.Release,
		vm.info.CPUs,
		vm.info.DiskUsage,
		vm.info.MemoryUsage,
		vm.info.Mounts,
	}

	var cells []string
	for i, val := range values {
		if i >= len(cols) {
			break
		}
		style := tableCellStyle.Width(cols[i].width)
		if selected {
			style = tableSelectedStyle.Width(cols[i].width)
		}
		// Color the State column
		if i == 1 && !selected {
			style = style.Foreground(stateColor(val))
		}
		// Truncate long values
		if len(val) > cols[i].width-2 && cols[i].width > 4 {
			val = val[:cols[i].width-4] + "…"
		}
		cells = append(cells, style.Render(val))
	}

	prefix := "  "
	if selected {
		prefix = tableCursorStyle.Render("▸ ")
	}

	return prefix + strings.Join(cells, "")
}

func (m tableModel) renderFooter() string {
	shortcuts := []struct{ key, desc string }{
		{"c", "Create"}, {"C", "Advanced"}, {"[", "Stop"}, {"]", "Start"},
		{"p", "Suspend"}, {"<", "StopAll"}, {">", "StartAll"}, {"i", "Info"},
	}
	shortcuts2 := []struct{ key, desc string }{
		{"d", "Delete"}, {"r", "Recover"}, {"!", "Purge"}, {"/", "Refresh"},
		{"f", "Filter"}, {"s", "Shell"}, {"n", "Snap"}, {"m", "Snaps"},
		{"M", "Mount"}, {"h", "Help"}, {"v", "Ver"}, {"q", "Quit"},
	}

	line1 := renderShortcutLine(shortcuts)
	line2 := renderShortcutLine(shortcuts2)

	sortHint := formHintStyle.Render(fmt.Sprintf("  Tab: cycle sort  Shift+Tab: toggle direction  (sorting by %s)", m.columns[m.sortColumn].title))

	return footerStyle.Render(line1 + "\n" + line2 + "\n" + sortHint)
}

func renderShortcutLine(shortcuts []struct{ key, desc string }) string {
	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, footerKeyStyle.Render(s.key)+" "+footerDescStyle.Render(s.desc))
	}
	return strings.Join(parts, "  ")
}
