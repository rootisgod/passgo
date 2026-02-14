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
	title    string
	width    int
	minWidth int  // smallest usable width before hiding
	priority int  // lower = more important, hidden last
	hidden   bool // set dynamically based on terminal width
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
			{title: "Name", width: 12, minWidth: 8, priority: 0},  // width set dynamically
			{title: "State", width: 12, minWidth: 10, priority: 0},
			{title: "Snaps", width: 7, minWidth: 5, priority: 5},
			{title: "IPv4", width: 16, minWidth: 12, priority: 2},
			{title: "CPU", width: 14, minWidth: 8, priority: 3},
			{title: "Disk", width: 18, minWidth: 8, priority: 3},
			{title: "Memory", width: 18, minWidth: 8, priority: 3},
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
	// title(1) + box_border(2) + header(1) + sep(1) + spacing(1) = 6
	used := 6
	if m.filterVisible {
		used++
	}
	// Footer lines vary by width
	if m.width >= 100 {
		used += 4 // 2 shortcut lines + status + sep
	} else if m.width >= 60 {
		used += 5 // 3 shortcut lines + status + sep
	} else {
		used += 3 // 1 shortcut line + status + sep
	}
	return max(1, m.height-used)
}

// ─── View ──────────────────────────────────────────────────────────────────────

func (m tableModel) View() string {
	var b strings.Builder

	// ── Title bar (full-width accent background, never wraps) ──
	vmCount := len(m.filteredVMs)
	totalCount := len(m.vms)
	countText := fmt.Sprintf(" %d VMs", totalCount)
	if vmCount != totalCount {
		countText = fmt.Sprintf(" %d/%d VMs", vmCount, totalCount)
	}
	liveIndicator := " ● LIVE"

	w := m.width
	if w < 1 {
		w = 80
	}

	// Build title bar content that fits within terminal width
	titleLabel := " ◆ Multipass"
	bgStyle := lipgloss.NewStyle().Background(accent)
	needed := lipgloss.Width(titleLabel) + lipgloss.Width(countText) + lipgloss.Width(liveIndicator)

	var titleText string
	if needed <= w {
		// Everything fits
		titleText = titleBarStyle.Render(titleLabel) +
			titleVMCountStyle.Render(countText) +
			titleLiveStyle.Render(liveIndicator)
	} else if lipgloss.Width(titleLabel)+lipgloss.Width(countText) <= w {
		// Drop the LIVE indicator
		titleText = titleBarStyle.Render(titleLabel) +
			titleVMCountStyle.Render(countText)
	} else {
		// Just the title
		titleText = titleBarStyle.Render(titleLabel)
	}

	// Pad to exactly terminal width (no overflow)
	titleVisibleWidth := lipgloss.Width(titleText)
	if w > titleVisibleWidth {
		pad := strings.Repeat(" ", w-titleVisibleWidth)
		titleText += bgStyle.Render(pad)
	}
	b.WriteString(titleText + "\n")

	// ── Filter bar ──
	if m.filterVisible {
		if m.filterFocused {
			b.WriteString(" " + filterIconStyle.Render("⌕ ") + m.filterInput.View() + "\n")
		} else {
			dim := " " + filterIconStyle.Render("⌕ ") + filterInactiveStyle.Render(m.filterText)
			b.WriteString(dim + "\n")
		}
	}

	// Compute dynamic column widths
	cols := m.computeColumnWidths()
	div := tableColDivStyle.Render("│")
	headerDiv := tableHeaderDivStyle.Render("│")

	// ── Header row with column dividers ──
	var headerCells []string
	first := true
	for i, col := range cols {
		if col.hidden {
			continue
		}
		title := col.title
		if i == m.sortColumn {
			if m.sortAscending {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}
		cell := tableHeaderStyle.Width(col.width).Render(title)
		if !first {
			cell = headerDiv + cell
		}
		first = false
		headerCells = append(headerCells, cell)
	}
	headerRow := " " + strings.Join(headerCells, "")

	// ── Header separator with column intersections ──
	var sepParts []string
	first = true
	for _, c := range cols {
		if c.hidden {
			continue
		}
		if !first {
			sepParts = append(sepParts, tableColDivStyle.Render("┼"))
		}
		first = false
		sepParts = append(sepParts, lipgloss.NewStyle().Foreground(dimmed).Render(strings.Repeat("─", c.width)))
	}
	sepRow := " " + strings.Join(sepParts, "")

	// ── Rows with column dividers ──
	visible := m.visibleRows()
	var rows []string
	if len(m.filteredVMs) == 0 {
		rows = append(rows, tableEmptyStyle.Render(" No VMs found"))
	} else {
		end := min(m.offset+visible, len(m.filteredVMs))
		for i := m.offset; i < end; i++ {
			vm := m.filteredVMs[i]
			selected := i == m.cursor
			rows = append(rows, m.renderRow(vm, cols, selected, div))
		}
	}

	// Pad remaining rows
	rendered := len(rows)
	for i := rendered; i < visible; i++ {
		rows = append(rows, "")
	}

	// ── Build table content inside a box ──
	tableContent := headerRow + "\n" + sepRow + "\n" + strings.Join(rows, "\n")

	// Calculate box width: sum of visible columns + dividers + padding
	tableWidth := 2 // left padding
	visibleCols := 0
	for _, c := range cols {
		if !c.hidden {
			tableWidth += c.width
			visibleCols++
		}
	}
	if visibleCols > 1 {
		tableWidth += visibleCols - 1 // divider characters
	}

	boxWidth := min(tableWidth+4, m.width-2)
	if boxWidth < 30 {
		boxWidth = 30
	}
	tableBox := tableBorderStyle.Width(boxWidth).Render(tableContent)
	b.WriteString(tableBox + "\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m tableModel) computeColumnWidths() []tableColumn {
	cols := make([]tableColumn, len(m.columns))
	copy(cols, m.columns)

	// Available width: terminal minus border(2) + prefix(1) + padding
	avail := m.width - 5

	// Reset hidden state
	for i := range cols {
		cols[i].hidden = false
	}

	// Size Name column to fit the longest VM name (+2 for padding)
	maxName := len("Name") // at least as wide as the header
	for _, vm := range m.filteredVMs {
		if l := lipgloss.Width(vm.info.Name); l > maxName {
			maxName = l
		}
	}
	nameWidth := maxName + 2
	if nameWidth < cols[0].minWidth {
		nameWidth = cols[0].minWidth
	}
	cols[0].width = nameWidth

	// Calculate total width at preferred sizes
	totalWidth := func() int {
		w := 0
		dividers := 0
		for _, c := range cols {
			if !c.hidden {
				w += c.width
				dividers++
			}
		}
		if dividers > 0 {
			dividers-- // n-1 dividers
		}
		return w + dividers
	}

	// Phase 1: Shrink columns to their minimum widths (lowest priority first)
	if totalWidth() > avail {
		for shrinkPri := 6; shrinkPri >= 0; shrinkPri-- {
			for i := range cols {
				if cols[i].hidden || cols[i].priority != shrinkPri {
					continue
				}
				if cols[i].width > cols[i].minWidth {
					cols[i].width = cols[i].minWidth
				}
			}
			if totalWidth() <= avail {
				break
			}
		}
	}

	// Phase 2: Hide columns by priority (highest number = least important)
	if totalWidth() > avail {
		for hidePri := 6; hidePri >= 1; hidePri-- {
			for i := range cols {
				if cols[i].priority == hidePri {
					cols[i].hidden = true
				}
			}
			if totalWidth() <= avail {
				break
			}
		}
	}

	// Phase 3: If wider than needed, distribute extra to resource columns and IPv4
	extra := avail - totalWidth()
	if extra > 0 && !cols[3].hidden {
		add := min(extra, 6) // IPv4
		cols[3].width += add
		extra -= add
	}
	// Spread remaining to CPU, Disk, Memory evenly
	for _, idx := range []int{4, 5, 6} {
		if extra <= 0 {
			break
		}
		if idx < len(cols) && !cols[idx].hidden {
			add := min(extra, 4)
			cols[idx].width += add
			extra -= add
		}
	}

	return cols
}

func (m tableModel) renderRow(vm vmData, cols []tableColumn, selected bool, div string) string {
	busy, isBusy := m.busyVMs[vm.info.Name]

	// Selection indicator: accent bar or space
	prefix := " "
	if selected {
		prefix = tableCursorStyle.Render("▎")
	}

	// Cell style helper
	cellStyle := func(width int) lipgloss.Style {
		if selected {
			return tableSelectedCellStyle.Width(width)
		}
		return tableCellStyle.Width(width)
	}

	// ── Busy row ──
	if isBusy {
		nameCell := cellStyle(cols[0].width).Render(vm.info.Name)

		progressWidth := 0
		for _, c := range cols[1:] {
			if !c.hidden {
				progressWidth += c.width
			}
		}

		phase := busy.phaseMessage()
		elapsed := busy.elapsed()
		barAvail := progressWidth - lipgloss.Width(phase) - lipgloss.Width(elapsed) - 6
		if barAvail < 4 {
			barAvail = 4
		}
		bar := renderProgressBar(busy.progressFraction(), barAvail)

		busyContent := m.spinner.View() + " " +
			lipgloss.NewStyle().Foreground(accent).Bold(true).Render(phase) + " " +
			bar + " " +
			lipgloss.NewStyle().Foreground(subtle).Italic(true).Render(elapsed)

		return prefix + nameCell + div + busyContent
	}

	// ── Normal row ──
	values := []string{
		vm.info.Name,
		vm.info.State,
		vm.info.Snapshots,
		vm.info.IPv4,
		"", // CPU
		"", // Disk
		"", // Memory
	}

	var cells []string
	first := true
	for i, val := range values {
		if i >= len(cols) {
			break
		}
		if cols[i].hidden {
			continue
		}

		style := cellStyle(cols[i].width)

		// Column divider
		cellDiv := ""
		if !first {
			cellDiv = div
		}
		first = false

		// State column
		if i == 1 {
			icon := stateIcon(val)
			if !selected {
				style = style.Foreground(stateColor(val))
			}
			cells = append(cells, cellDiv+style.Render(icon+" "+val))
			continue
		}

		// CPU column (index 4)
		if i == 4 {
			barW := cols[i].width - 6
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseCPULoadFraction(vm.info.Load, vm.info.CPUs); ok {
				cells = append(cells, cellDiv+style.Render(renderSparkBar(frac, barW, usageBarColor(frac))))
			} else {
				cells = append(cells, cellDiv+style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.CPUs)))
			}
			continue
		}

		// Disk column (index 5)
		if i == 5 {
			barW := cols[i].width - 6
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseUsageFraction(vm.info.DiskUsage); ok {
				cells = append(cells, cellDiv+style.Render(renderSparkBar(frac, barW, usageBarColor(frac))))
			} else {
				cells = append(cells, cellDiv+style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.DiskUsage)))
			}
			continue
		}

		// Memory column (index 6)
		if i == 6 {
			barW := cols[i].width - 6
			if barW < 3 {
				barW = 3
			}
			if frac, ok := parseUsageFraction(vm.info.MemoryUsage); ok {
				cells = append(cells, cellDiv+style.Render(renderSparkBar(frac, barW, usageBarColor(frac))))
			} else {
				cells = append(cells, cellDiv+style.Render(lipgloss.NewStyle().Foreground(subtle).Render(vm.info.MemoryUsage)))
			}
			continue
		}

		// Default: truncate and render
		visibleLen := lipgloss.Width(val)
		if visibleLen > cols[i].width-2 && cols[i].width > 4 {
			val = val[:cols[i].width-4] + "…"
		}
		cells = append(cells, cellDiv+style.Render(val))
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

	divider := footerSepStyle.Render("  │  ")

	// Responsive footer: adjust based on terminal width
	var footerLines string
	if m.width >= 100 {
		// Two lines with groups
		line1 := renderShortcutLine(vmOps) + divider + renderShortcutLine(bulkOps)
		line2 := renderShortcutLine(navOps) + divider + renderShortcutLine(appOps)
		footerLines = line1 + "\n" + line2
	} else if m.width >= 60 {
		// Compact: all on separate lines
		line1 := renderShortcutLine(vmOps)
		line2 := renderShortcutLine(bulkOps) + divider + renderShortcutLine(navOps)
		line3 := renderShortcutLine(appOps)
		footerLines = line1 + "\n" + line2 + "\n" + line3
	} else {
		// Very narrow: minimal shortcuts
		essentials := []struct{ key, desc string }{
			{"c", "Create"}, {"[", "Stop"}, {"]", "Start"},
			{"i", "Info"}, {"s", "Shell"}, {"q", "Quit"},
		}
		footerLines = renderShortcutLine(essentials)
	}

	// Status line: sort info + refresh info (truncated to fit)
	sortDir := "▲ asc"
	if !m.sortAscending {
		sortDir = "▼ desc"
	}

	var statusContent string
	if m.width >= 60 {
		statusContent = fmt.Sprintf("  Tab: sort by %s  Shift+Tab: %s",
			m.columns[m.sortColumn].title, sortDir)
		if !m.lastRefresh.IsZero() {
			ago := time.Since(m.lastRefresh).Truncate(time.Second)
			statusContent += fmt.Sprintf("  ·  ↻ %s ago", ago)
		}
	} else {
		statusContent = fmt.Sprintf("  Sort: %s %s",
			m.columns[m.sortColumn].title, sortDir)
	}
	statusLine := formHintStyle.Render(statusContent)

	return sep + "\n" + footerStyle.Render(footerLines + "\n" + statusLine)
}

func renderShortcutLine(shortcuts []struct{ key, desc string }) string {
	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, footerKeyStyle.Render(s.key)+" "+footerDescStyle.Render(s.desc))
	}
	return strings.Join(parts, "  ")
}
