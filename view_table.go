// view_table.go - Main VM table with filter bar, sorting, and footer
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Column definitions for the VM table.
type tableColumn struct {
	title string
	width int
}

// busyInfo tracks an in-flight inline operation for a VM.
type busyInfo struct {
	operation string    // "Stopping", "Starting", "Suspending", "Recovering"
	startTime time.Time // when the operation began
}

// phaseMessage returns a context-aware status message based on elapsed time.
func (b busyInfo) phaseMessage() string {
	elapsed := time.Since(b.startTime)
	switch {
	case elapsed < 3*time.Second:
		return b.operation + "…"
	case elapsed < 8*time.Second:
		return "Working on it…"
	case elapsed < 15*time.Second:
		return "Almost there…"
	default:
		return "Hang tight…"
	}
}

// elapsed returns the seconds since the operation started.
func (b busyInfo) elapsed() string {
	secs := int(time.Since(b.startTime).Seconds())
	return fmt.Sprintf("%ds", secs)
}

// progressFraction returns a fake progress (0.0–0.95) using a log curve.
// It approaches but never reaches 1.0 until the real operation completes.
func (b busyInfo) progressFraction() float64 {
	secs := time.Since(b.startTime).Seconds()
	// Logarithmic curve: rises quickly then asymptotes near 0.95
	p := 0.95 * (1 - 1/(1+secs/5))
	if p > 0.95 {
		p = 0.95
	}
	return p
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

	// Inline operation tracking
	busyVMs map[string]busyInfo
	spinner spinner.Model

	// Auto-refresh
	lastRefresh time.Time
}

func newTableModel() tableModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Prompt = "Filter: "
	ti.PromptStyle = filterActiveStyle
	ti.CharLimit = 64

	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		FPS:    80 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(accent).Bold(true)

	return tableModel{
		filterInput:   ti,
		sortColumn:    0,
		sortAscending: true,
		busyVMs:       make(map[string]busyInfo),
		spinner:       s,
		columns: []tableColumn{
			{title: "Name", width: 18},
			{title: "State", width: 12},
			{title: "Snaps", width: 7},
			{title: "IPv4", width: 16},
			{title: "Release", width: 22},
			{title: "CPU", width: 14},
			{title: "Disk", width: 18},
			{title: "Memory", width: 18},
			{title: "Mounts", width: 8},
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
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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
	// title(1) + header(1) + sep(1) + footer_sep(1) + footer(3) + spacing(1)
	used := 8
	if m.filterVisible {
		used++
	}
	return max(1, m.height-used)
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m tableModel) View() string {
	var b strings.Builder

	// ── Title bar (full-width accent background) ──
	vmCount := len(m.filteredVMs)
	totalCount := len(m.vms)
	countText := fmt.Sprintf(" %d VMs", totalCount)
	if vmCount != totalCount {
		countText = fmt.Sprintf(" %d/%d VMs", vmCount, totalCount)
	}
	liveIndicator := titleLiveStyle.Render(" ● LIVE")
	titleText := titleBarStyle.Render(" ◆ Multipass") +
		titleVMCountStyle.Render(countText) +
		liveIndicator

	// Pad the title bar to full terminal width
	titleVisibleWidth := lipgloss.Width(titleText)
	if m.width > titleVisibleWidth {
		pad := strings.Repeat(" ", m.width-titleVisibleWidth)
		titleText += lipgloss.NewStyle().Background(accent).Render(pad)
	}
	b.WriteString(titleText + "\n")

	// ── Filter bar ──
	if m.filterVisible {
		if m.filterFocused {
			b.WriteString("  " + filterIconStyle.Render("⌕ ") + m.filterInput.View() + "\n")
		} else {
			dim := "  " + filterIconStyle.Render("⌕ ") + filterInactiveStyle.Render(m.filterText)
			b.WriteString(dim + "\n")
		}
	}

	// Compute dynamic column widths
	cols := m.computeColumnWidths()

	// ── Header row ──
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

	// ── Separator ──
	sepLen := 0
	for _, c := range cols {
		sepLen += c.width
	}
	sep := lipgloss.NewStyle().Foreground(dimmed).Render(strings.Repeat("─", min(sepLen+2, m.width)))
	b.WriteString("  " + sep + "\n")

	// ── Rows ──
	visible := m.visibleRows()
	if len(m.filteredVMs) == 0 {
		b.WriteString(tableEmptyStyle.Render("  No VMs found") + "\n")
	} else {
		end := min(m.offset+visible, len(m.filteredVMs))
		for i := m.offset; i < end; i++ {
			vm := m.filteredVMs[i]
			selected := i == m.cursor
			isAlt := (i-m.offset)%2 == 1
			b.WriteString(m.renderRow(vm, cols, selected, isAlt) + "\n")
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

func (m tableModel) renderRow(vm vmData, cols []tableColumn, selected bool, isAlt bool) string {
	busy, isBusy := m.busyVMs[vm.info.Name]

	prefix := "  "
	if selected {
		prefix = tableCursorStyle.Render("▸ ")
	}

	// ── Busy row: Name + animated progress bar spanning remaining columns ──
	if isBusy {
		// Render name column
		nameStyle := tableCellStyle.Width(cols[0].width)
		if selected {
			nameStyle = tableSelectedStyle.Width(cols[0].width)
		}
		nameCell := nameStyle.Render(vm.info.Name)

		// Calculate width for the progress area (all columns after Name)
		progressWidth := 0
		for _, c := range cols[1:] {
			progressWidth += c.width
		}

		// Build status components
		phase := busy.phaseMessage()
		elapsed := busy.elapsed()

		// Calculate bar width: total area minus spinner, phase text, elapsed, spacing
		barAvail := progressWidth - lipgloss.Width(phase) - lipgloss.Width(elapsed) - 6
		if barAvail < 8 {
			barAvail = 8
		}
		bar := renderProgressBar(busy.progressFraction(), barAvail)

		busyContent := m.spinner.View() + " " +
			lipgloss.NewStyle().Foreground(accent).Bold(true).Render(phase) + " " +
			bar + " " +
			lipgloss.NewStyle().Foreground(subtle).Italic(true).Render(elapsed)

		return prefix + nameCell + busyContent
	}

	// ── Normal row ──
	values := []string{
		vm.info.Name,
		vm.info.State,
		vm.info.Snapshots,
		vm.info.IPv4,
		vm.info.Release,
		"", // CPU — rendered as spark bar
		"", // Disk — rendered as spark bar
		"", // Memory — rendered as spark bar
		vm.info.Mounts,
	}

	var cells []string
	for i, val := range values {
		if i >= len(cols) {
			break
		}

		// Pick base style: selected > alt-row > normal
		style := tableCellStyle.Width(cols[i].width)
		if selected {
			style = tableSelectedStyle.Width(cols[i].width)
		} else if isAlt {
			style = tableCellAltStyle.Width(cols[i].width)
		}

		// State column: add colored dot icon
		if i == 1 {
			icon := stateIcon(val)
			if !selected {
				style = style.Foreground(stateColor(val))
			}
			stateText := icon + " " + val
			cells = append(cells, style.Render(stateText))
			continue
		}

		// CPU column (index 5): spark bar from load average
		if i == 5 {
			barW := cols[i].width - 6 // leave room for "XXX%" text
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseCPULoadFraction(vm.info.Load, vm.info.CPUs); ok {
				sparkBar := renderSparkBar(frac, barW, usageBarColor(frac))
				cells = append(cells, style.Render(sparkBar))
			} else {
				cells = append(cells, style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.CPUs)))
			}
			continue
		}

		// Disk column (index 6): spark bar
		if i == 6 {
			barW := cols[i].width - 6
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseUsageFraction(vm.info.DiskUsage); ok {
				sparkBar := renderSparkBar(frac, barW, usageBarColor(frac))
				cells = append(cells, style.Render(sparkBar))
			} else {
				cells = append(cells, style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.DiskUsage)))
			}
			continue
		}

		// Memory column (index 7): spark bar
		if i == 7 {
			barW := cols[i].width - 6
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseUsageFraction(vm.info.MemoryUsage); ok {
				sparkBar := renderSparkBar(frac, barW, usageBarColor(frac))
				cells = append(cells, style.Render(sparkBar))
			} else {
				cells = append(cells, style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.MemoryUsage)))
			}
			continue
		}

		// Truncate long values (safe for plain text only)
		visibleLen := lipgloss.Width(val)
		if visibleLen > cols[i].width-2 && cols[i].width > 4 {
			val = val[:cols[i].width-4] + "…"
		}
		cells = append(cells, style.Render(val))
	}

	return prefix + strings.Join(cells, "")
}

// ─── Usage Bars ─────────────────────────────────────────────────────────────────

// parseUsageFraction parses "X.XGiB out of Y.YGiB" or "X.XMiB out of Y.YMiB" into 0.0–1.0.
func parseUsageFraction(usage string) (float64, bool) {
	if usage == "" || usage == "--" {
		return 0, false
	}
	parts := strings.SplitN(usage, " out of ", 2)
	if len(parts) != 2 {
		return 0, false
	}
	used := parseSize(strings.TrimSpace(parts[0]))
	total := parseSize(strings.TrimSpace(parts[1]))
	if total <= 0 {
		return 0, false
	}
	frac := used / total
	if frac > 1 {
		frac = 1
	}
	return frac, true
}

// parseSize converts "2.5GiB" or "228.6MiB" to a float in MiB.
func parseSize(s string) float64 {
	s = strings.TrimSpace(s)
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	switch {
	case strings.HasPrefix(unit, "GiB"), strings.HasPrefix(unit, "G"):
		return val * 1024
	case strings.HasPrefix(unit, "MiB"), strings.HasPrefix(unit, "M"):
		return val
	case strings.HasPrefix(unit, "KiB"), strings.HasPrefix(unit, "K"):
		return val / 1024
	}
	return val
}

// parseCPULoadFraction parses load average and CPU count into a 0.0–1.0 fraction.
func parseCPULoadFraction(load string, cpus string) (float64, bool) {
	if load == "" || load == "--" || cpus == "" || cpus == "--" {
		return 0, false
	}
	fields := strings.Fields(load)
	if len(fields) == 0 {
		return 0, false
	}
	var loadVal float64
	fmt.Sscanf(fields[0], "%f", &loadVal)
	var cpuCount float64
	fmt.Sscanf(cpus, "%f", &cpuCount)
	if cpuCount <= 0 {
		cpuCount = 1
	}
	frac := loadVal / cpuCount
	if frac > 1 {
		frac = 1
	}
	return frac, true
}

// renderSparkBar draws a compact bar: ▓▓▓▓░░░░ 52%
func renderSparkBar(fraction float64, barWidth int, clr lipgloss.Color) string {
	if barWidth < 2 {
		barWidth = 2
	}
	filled := int(fraction * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		bar.WriteString("▓")
	}
	for i := filled; i < barWidth; i++ {
		bar.WriteString("░")
	}

	pct := int(fraction * 100)
	pctStr := fmt.Sprintf("%3d%%", pct)

	return lipgloss.NewStyle().Foreground(clr).Render(bar.String()) +
		lipgloss.NewStyle().Foreground(subtle).Render(pctStr)
}

// usageBarColor returns a color based on usage percentage (green→amber→red).
func usageBarColor(fraction float64) lipgloss.Color {
	switch {
	case fraction < 0.6:
		return runningClr // green
	case fraction < 0.85:
		return suspendClr // amber
	default:
		return stoppedClr // red
	}
}

// renderProgressBar builds a Unicode block progress bar at the given fraction (0.0–1.0).
func renderProgressBar(fraction float64, width int) string {
	if width < 2 {
		width = 2
	}

	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}

	// Block characters for smooth sub-character progress
	blocks := []string{"░", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

	// Calculate partial block
	fractionalPart := (fraction * float64(width)) - float64(filled)
	partialIdx := int(fractionalPart * float64(len(blocks)-1))
	if partialIdx >= len(blocks) {
		partialIdx = len(blocks) - 1
	}

	var bar strings.Builder
	// Filled portion
	for i := 0; i < filled; i++ {
		bar.WriteString("█")
	}
	// Partial block
	if filled < width {
		bar.WriteString(blocks[partialIdx])
		filled++
	}
	// Empty portion
	for i := filled; i < width; i++ {
		bar.WriteString("░")
	}

	barStr := bar.String()

	return lipgloss.NewStyle().Foreground(accent).Render(barStr)
}

func (m tableModel) renderFooter() string {
	// Separator line
	sepWidth := m.width
	if sepWidth < 20 {
		sepWidth = 80
	}
	sep := footerSepStyle.Render(strings.Repeat("─", sepWidth))

	// Group shortcuts by category
	vmOps := []struct{ key, desc string }{
		{"c", "Create"}, {"C", "Adv Create"}, {"[", "Stop"}, {"]", "Start"},
		{"p", "Suspend"}, {"d", "Delete"}, {"r", "Recover"},
	}
	bulkOps := []struct{ key, desc string }{
		{"<", "StopAll"}, {">", "StartAll"}, {"!", "Purge"},
	}
	navOps := []struct{ key, desc string }{
		{"i", "Info"}, {"s", "Shell"}, {"n", "Snap"}, {"m", "Snaps"}, {"M", "Mount"},
	}
	appOps := []struct{ key, desc string }{
		{"f", "Filter"}, {"/", "Refresh"}, {"h", "Help"}, {"v", "Ver"}, {"q", "Quit"},
	}

	line1 := renderShortcutLine(vmOps) +
		footerSepStyle.Render("  │  ") +
		renderShortcutLine(bulkOps)
	line2 := renderShortcutLine(navOps) +
		footerSepStyle.Render("  │  ") +
		renderShortcutLine(appOps)

	// Status line: sort info + refresh info
	sortInfo := fmt.Sprintf("Tab: sort by %s  Shift+Tab: %s",
		m.columns[m.sortColumn].title,
		func() string {
			if m.sortAscending {
				return "▲ asc"
			}
			return "▼ desc"
		}())

	var refreshInfo string
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Truncate(time.Second)
		refreshInfo = fmt.Sprintf("  ·  ↻ updated %s ago", ago)
	}
	statusLine := formHintStyle.Render("  " + sortInfo + refreshInfo)

	return sep + "\n" + footerStyle.Render(line1 + "\n" + line2 + "\n" + statusLine)
}

func renderShortcutLine(shortcuts []struct{ key, desc string }) string {
	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, footerKeyStyle.Render(s.key)+" "+footerDescStyle.Render(s.desc))
	}
	return strings.Join(parts, "  ")
}
