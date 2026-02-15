// view_info.go - Scrollable VM details view with live resource charts
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	infoRefreshInterval = 1 * time.Second
	sparkHistoryLen     = 40 // number of data points in the sparkline
)

// sparkline characters from lowest to highest (8 levels).
var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type infoModel struct {
	vmName   string
	vmState  string
	viewport viewport.Model
	content  string
	ready    bool
	width    int
	height   int

	// Live resource tracking
	cpuHistory  []float64
	diskHistory []float64
	memHistory  []float64
	lastCPU     float64
	lastDisk    float64
	lastMem     float64
	lastCPUs    string
	lastLoad    string
	lastDiskRaw string
	lastMemRaw  string
}

func newInfoModel(vmName string, width, height int) infoModel {
	return infoModel{
		vmName:      vmName,
		width:       width,
		height:      height,
		cpuHistory:  make([]float64, 0, sparkHistoryLen),
		diskHistory: make([]float64, 0, sparkHistoryLen),
		memHistory:  make([]float64, 0, sparkHistoryLen),
	}
}

func (m *infoModel) setContent(raw string) {
	// Parse resource data from the raw info
	var cpus, load, diskRaw, memRaw, state string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "State":
				state = val
			case "CPU(s)":
				cpus = val
			case "Load":
				load = val
			case "Disk usage":
				diskRaw = val
			case "Memory usage":
				memRaw = val
			}
		}
	}

	m.vmState = state
	m.lastCPUs = cpus
	m.lastLoad = load
	m.lastDiskRaw = diskRaw
	m.lastMemRaw = memRaw

	// Update history
	if frac, ok := parseCPULoadFraction(load, cpus); ok {
		m.lastCPU = frac
		m.cpuHistory = appendHistory(m.cpuHistory, frac)
	}
	if frac, ok := parseUsageFraction(diskRaw); ok {
		m.lastDisk = frac
		m.diskHistory = appendHistory(m.diskHistory, frac)
	}
	if frac, ok := parseUsageFraction(memRaw); ok {
		m.lastMem = frac
		m.memHistory = appendHistory(m.memHistory, frac)
	}

	// Build formatted info content (excluding the chart fields, they go above)
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
	vpHeight := m.height - 18 // leave room for charts at top
	if vpHeight < 5 {
		vpHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
	m.viewport.SetContent(m.content)
}

func appendHistory(history []float64, val float64) []float64 {
	history = append(history, val)
	if len(history) > sparkHistoryLen {
		history = history[len(history)-sparkHistoryLen:]
	}
	return history
}

func (m infoModel) Update(msg tea.Msg) (infoModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter", "q":
			return m, func() tea.Msg { return backToTableMsg{} }
		}

	case infoRefreshTickMsg:
		// Only refresh if on a running VM
		if m.vmState == "Running" {
			return m, tea.Batch(
				infoRefreshTickCmd(),
				fetchVMInfoCmd(m.vmName),
			)
		}
		// Still schedule the next tick in case state changes
		return m, infoRefreshTickCmd()

	case vmInfoResultMsg:
		if msg.err == nil {
			m.setContent(msg.info)
		}
		return m, nil
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func infoRefreshTickCmd() tea.Cmd {
	return tea.Tick(infoRefreshInterval, func(t time.Time) tea.Msg {
		return infoRefreshTickMsg(t)
	})
}

func (m infoModel) View() string {
	title := modalTitleStyle.Render(fmt.Sprintf("Info: %s", m.vmName))

	// Live resource charts
	charts := m.renderCharts()

	var body string
	if !m.ready {
		body = loadingMsgStyle.Render("Loading…")
	} else {
		body = m.viewport.View()
	}

	scrollHint := ""
	if m.ready {
		pct := m.viewport.ScrollPercent()
		scrollHint = lipgloss.NewStyle().Foreground(subtle).Render(
			fmt.Sprintf(" %.0f%%", pct*100))
	}

	hint := formHintStyle.Render("↑↓ scroll  Esc close") + scrollHint
	content := title + "\n\n" + charts + "\n" + body + "\n\n" + hint

	box := infoBorderStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m infoModel) renderCharts() string {
	if m.vmState != "Running" && len(m.cpuHistory) == 0 {
		return lipgloss.NewStyle().Foreground(subtle).Italic(true).Render(
			"  Resource charts available when VM is running")
	}

	chartWidth := min(m.width-12, 80)
	barWidth := chartWidth - 22 // leave room for label + percentage + detail
	if barWidth < 10 {
		barWidth = 10
	}

	// Build detail strings showing actual values
	cpuDetail := formatCPUDetail(m.lastLoad, m.lastCPUs)
	memDetail := formatUsageDetail(m.lastMemRaw)
	diskDetail := formatUsageDetail(m.lastDiskRaw)

	// CPU chart
	cpuLine := renderChartLine("CPU", m.lastCPU, cpuDetail, m.cpuHistory, barWidth)
	// Memory chart
	memLine := renderChartLine("Mem", m.lastMem, memDetail, m.memHistory, barWidth)
	// Disk chart
	diskLine := renderChartLine("Dsk", m.lastDisk, diskDetail, m.diskHistory, barWidth)

	sep := lipgloss.NewStyle().Foreground(dimmed).Render(strings.Repeat("─", chartWidth))

	return cpuLine + "\n" + memLine + "\n" + diskLine + "\n" + sep
}

// formatCPUDetail returns e.g. "0.12 load / 2 CPUs"
func formatCPUDetail(load, cpus string) string {
	if load == "" || load == "--" {
		return cpus + " CPUs"
	}
	fields := strings.Fields(load)
	loadVal := load
	if len(fields) > 0 {
		loadVal = fields[0]
	}
	return loadVal + " load / " + cpus + " CPUs"
}

// formatUsageDetail returns e.g. "1.2GiB / 3.8GiB"
func formatUsageDetail(raw string) string {
	if raw == "" || raw == "--" {
		return "--"
	}
	parts := strings.SplitN(raw, " out of ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]) + " / " + strings.TrimSpace(parts[1])
	}
	return raw
}

func renderChartLine(label string, fraction float64, detail string, history []float64, barWidth int) string {
	clr := usageBarColor(fraction)
	pct := int(fraction * 100)

	// Label
	labelStr := lipgloss.NewStyle().Foreground(accent).Bold(true).Width(4).Render(label)

	// Current bar
	currentBarWidth := 8
	currentBar := renderMiniBar(fraction, currentBarWidth, clr)

	// Percentage
	pctStr := lipgloss.NewStyle().Foreground(clr).Bold(true).Width(5).Render(
		fmt.Sprintf("%3d%%", pct))

	// Detail string (actual values)
	detailStr := lipgloss.NewStyle().Foreground(subtle).Width(22).Render(detail)

	// Sparkline history
	sparkWidth := barWidth - currentBarWidth - 22
	if sparkWidth < 5 {
		sparkWidth = 5
	}
	spark := renderSparkline(history, sparkWidth, clr)

	return "  " + labelStr + currentBar + " " + pctStr + " " + detailStr + " " + spark
}

// renderMiniBar draws a compact filled/empty bar.
func renderMiniBar(fraction float64, width int, clr lipgloss.Color) string {
	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}

	var b strings.Builder
	for i := 0; i < filled; i++ {
		b.WriteString("█")
	}
	for i := filled; i < width; i++ {
		b.WriteString("░")
	}
	return lipgloss.NewStyle().Foreground(clr).Render(b.String())
}

// renderSparkline draws a sparkline from historical data using Unicode block chars.
func renderSparkline(data []float64, width int, clr lipgloss.Color) string {
	if len(data) == 0 {
		return lipgloss.NewStyle().Foreground(dimmed).Render(strings.Repeat("⎯", width))
	}

	// Pad or trim data to fit width
	display := make([]float64, width)
	if len(data) >= width {
		// Take the most recent `width` values
		copy(display, data[len(data)-width:])
	} else {
		// Pad the left with zeros
		offset := width - len(data)
		for i := 0; i < offset; i++ {
			display[i] = 0
		}
		copy(display[offset:], data)
	}

	var b strings.Builder
	for _, v := range display {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		idx := int(v * float64(len(sparkChars)-1))
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[idx])
	}

	return lipgloss.NewStyle().Foreground(clr).Render(b.String())
}
