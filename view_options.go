// view_options.go - VM Options form for modifying CPU, RAM, and Disk
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type optionsModel struct {
	vmName        string
	cpuInput      textinput.Model
	memInput      textinput.Model
	diskInput     textinput.Model
	cursor        int // 0=cpu, 1=mem, 2=disk, 3=save, 4=cancel
	width         int
	height        int
	config        ResourceConfig
	limits        HostLimits
	currentDiskGB int    // Track current disk for validation
	validationErr string // Validation error message to display
}

func newOptionsModel(vmName string, config ResourceConfig, limits HostLimits, width, height int) optionsModel {
	// Parse current values
	currentMemMB, _ := parseMemoryToMB(config.Memory)
	currentDiskGB, _ := parseDiskToGB(config.Disk)

	cpuInput := textinput.New()
	cpuInput.SetValue(strconv.Itoa(config.CPUs))
	cpuInput.Focus()
	cpuInput.CharLimit = 4

	memInput := textinput.New()
	memInput.SetValue(strconv.Itoa(currentMemMB))
	memInput.CharLimit = 8

	diskInput := textinput.New()
	diskInput.SetValue(strconv.Itoa(currentDiskGB))
	diskInput.CharLimit = 6

	return optionsModel{
		vmName:        vmName,
		cpuInput:      cpuInput,
		memInput:      memInput,
		diskInput:     diskInput,
		cursor:        0,
		width:         width,
		height:        height,
		config:         config,
		limits:        limits,
		currentDiskGB:  currentDiskGB,
	}
}

func (m optionsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m optionsModel) Update(msg tea.Msg) (optionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return backToTableMsg{} }

		case "tab", "down":
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % 5
			m.focusCurrent()
			return m, nil

		case "shift+tab", "up":
			m.blurCurrent()
			m.cursor = (m.cursor - 1 + 5) % 5
			m.focusCurrent()
			return m, nil

		case "left":
			m2, cmd := m.adjustValue(-1)
			return m2, cmd

		case "right":
			m2, cmd := m.adjustValue(1)
			return m2, cmd

		case "enter":
			if m.cursor == 4 { // Cancel
				return m, func() tea.Msg { return backToTableMsg{} }
			}
			if m.cursor == 3 { // Save
				return m.submit()
			}
			// Move to next field
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % 5
			m.focusCurrent()
			return m, nil
		}

		// Text input for current field
		if m.cursor >= 0 && m.cursor <= 2 {
			// Clear validation error when user starts typing
			m.validationErr = ""
			var cmd tea.Cmd
			switch m.cursor {
			case 0:
				m.cpuInput, cmd = m.cpuInput.Update(msg)
			case 1:
				m.memInput, cmd = m.memInput.Update(msg)
			case 2:
				m.diskInput, cmd = m.diskInput.Update(msg)
			}
			return m, cmd
		}
	}

	// Pass tick messages to focused textinput for cursor blink
	if m.cursor >= 0 && m.cursor <= 2 {
		var cmd tea.Cmd
		switch m.cursor {
		case 0:
			m.cpuInput, cmd = m.cpuInput.Update(msg)
		case 1:
			m.memInput, cmd = m.memInput.Update(msg)
		case 2:
			m.diskInput, cmd = m.diskInput.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

func (m *optionsModel) blurCurrent() {
	switch m.cursor {
	case 0:
		m.cpuInput.Blur()
	case 1:
		m.memInput.Blur()
	case 2:
		m.diskInput.Blur()
	}
}

func (m *optionsModel) focusCurrent() {
	switch m.cursor {
	case 0:
		m.cpuInput.Focus()
	case 1:
		m.memInput.Focus()
	case 2:
		m.diskInput.Focus()
	}
}

// adjustValue adjusts the current field value by delta
func (m *optionsModel) adjustValue(delta int) (optionsModel, tea.Cmd) {
	switch m.cursor {
	case 0: // CPU
		val, err := strconv.Atoi(m.cpuInput.Value())
		if err == nil {
			newVal := val + delta
			if newVal >= MinCPUCores && newVal <= m.limits.MaxCPUs {
				m.cpuInput.SetValue(strconv.Itoa(newVal))
			}
		}
	case 1: // Memory
		val, err := strconv.Atoi(m.memInput.Value())
		if err == nil {
			newVal := val + (delta * 256) // Adjust by 256MB chunks
			if newVal >= MinRAMMB && newVal <= m.limits.MaxMemory {
				m.memInput.SetValue(strconv.Itoa(newVal))
			}
		}
	case 2: // Disk
		val, err := strconv.Atoi(m.diskInput.Value())
		if err == nil {
			newVal := val + delta // Adjust by 1GB chunks
			if newVal >= m.currentDiskGB && newVal >= MinDiskGB {
				m.diskInput.SetValue(strconv.Itoa(newVal))
			}
		}
	}
	return *m, nil
}

func (m *optionsModel) validate() bool {
	m.validationErr = ""

	// Validate CPU
	cpus, err := strconv.Atoi(m.cpuInput.Value())
	if err != nil || cpus < MinCPUCores {
		m.validationErr = fmt.Sprintf("CPU must be at least %d", MinCPUCores)
		return false
	}
	if cpus > m.limits.MaxCPUs {
		m.validationErr = fmt.Sprintf("CPU cannot exceed %d", m.limits.MaxCPUs)
		return false
	}

	// Validate Memory
	memMB, err := strconv.Atoi(m.memInput.Value())
	if err != nil || memMB < MinRAMMB {
		m.validationErr = fmt.Sprintf("RAM must be at least %d MB", MinRAMMB)
		return false
	}
	if memMB > m.limits.MaxMemory {
		m.validationErr = fmt.Sprintf("RAM cannot exceed %d MB", m.limits.MaxMemory)
		return false
	}

	// Validate Disk
	diskGB, err := strconv.Atoi(m.diskInput.Value())
	if err != nil || diskGB < MinDiskGB {
		m.validationErr = fmt.Sprintf("Disk must be at least %d GB", MinDiskGB)
		return false
	}
	if diskGB < m.currentDiskGB {
		m.validationErr = fmt.Sprintf("Disk cannot be smaller than current %d GB", m.currentDiskGB)
		return false
	}

	return true
}

func (m *optionsModel) submit() (optionsModel, tea.Cmd) {
	// Validate first
	if !m.validate() {
		// Return the model with error displayed
		return *m, nil
	}

	cpus, _ := strconv.Atoi(m.cpuInput.Value())
	memMB, _ := strconv.Atoi(m.memInput.Value())
	diskGB, _ := strconv.Atoi(m.diskInput.Value())
	vmName := m.vmName

	newConfig := ResourceConfig{
		CPUs:   cpus,
		Memory: formatMemoryToMiB(memMB),
		Disk:   formatDiskToGiB(diskGB),
	}

	return *m, func() tea.Msg {
		err := setVMResourceConfig(vmName, newConfig)
		return vmOperationResultMsg{vmName: vmName, operation: "set-options", err: err}
	}
}

func (m optionsModel) View() string {
	// Title
	titleLabel := " ◆ VM Options: " + m.vmName
	w := min(m.width-4, 60)
	if w < 30 {
		w = 30
	}
	titleText := titleBarStyle.Render(titleLabel)
	titleVisibleWidth := lipgloss.Width(titleText)
	if w > titleVisibleWidth {
		pad := strings.Repeat(" ", w-titleVisibleWidth)
		titleText += lipgloss.NewStyle().Background(accent).Render(pad)
	}

	// Column widths
	labelW := 14
	valueW := w - labelW - 5
	if valueW < 16 {
		valueW = 16
	}

	divStyle := lipgloss.NewStyle().Foreground(dimmed)
	div := divStyle.Render("│")

	// Header
	headerLabel := tableHeaderStyle.Width(labelW).Render("Field")
	headerValue := tableHeaderStyle.Width(valueW).Render("Value")
	header := "  " + headerLabel + div + headerValue

	// Separator
	sep := "  " + divStyle.Render(strings.Repeat("─", labelW)) +
		divStyle.Render("┼") +
		divStyle.Render(strings.Repeat("─", valueW))

	// Rows
	var rows []string
	var buttons []string

	// CPU row
	cpuActive := m.cursor == 0
	cpuPrefix := "  "
	if cpuActive {
		cpuPrefix = tableCursorStyle.Render("▎ ")
	}
	cpuLabelStyle := formLabelStyle.Width(labelW)
	if cpuActive {
		cpuLabelStyle = formActiveLabelStyle.Width(labelW)
	}
	cpuLabel := cpuLabelStyle.Render("CPU Cores")
	cpuVal := m.renderNumericField(m.cpuInput, cpuActive, m.limits.MaxCPUs)

	// Memory row
	memActive := m.cursor == 1
	memPrefix := "  "
	if memActive {
		memPrefix = tableCursorStyle.Render("▎ ")
	}
	memLabelStyle := formLabelStyle.Width(labelW)
	if memActive {
		memLabelStyle = formActiveLabelStyle.Width(labelW)
	}
	memLabel := memLabelStyle.Render("RAM (MB)")
	memVal := m.renderNumericField(m.memInput, memActive, m.limits.MaxMemory)

	// Disk row
	diskActive := m.cursor == 2
	diskPrefix := "  "
	if diskActive {
		diskPrefix = tableCursorStyle.Render("▎ ")
	}
	diskLabelStyle := formLabelStyle.Width(labelW)
	if diskActive {
		diskLabelStyle = formActiveLabelStyle.Width(labelW)
	}
	diskLabel := diskLabelStyle.Render("Disk (GB)")
	diskVal := m.renderNumericField(m.diskInput, diskActive, 0) // Disk has different validation

	rows = append(rows, cpuPrefix+cpuLabel+div+cpuVal)
	rows = append(rows, memPrefix+memLabel+div+memVal)
	rows = append(rows, diskPrefix+diskLabel+div+diskVal)

	// Buttons
	saveActive := m.cursor == 3
	cancelActive := m.cursor == 4
	saveStyle := formButtonStyle
	cancelStyle := formButtonStyle
	if saveActive {
		saveStyle = formActiveButtonStyle
	}
	if cancelActive {
		cancelStyle = formActiveButtonStyle
	}
	buttons = append(buttons, saveStyle.Render("[ Save ]"))
	buttons = append(buttons, cancelStyle.Render("[ Cancel ]"))

	// Build table inside a border box
	tableContent := header + "\n" + sep + "\n" + strings.Join(rows, "\n")
	tableBox := tableBorderStyle.Width(w + 2).Render(tableContent)

	// Details panel
	maxMemGB := m.limits.MaxMemory / 1024
	detailsText := fmt.Sprintf("Host Limits: Max %d CPUs, Max %d MB (~%d GB)\n\nNote: Disk size cannot be smaller than current allocation.",
		m.limits.MaxCPUs, m.limits.MaxMemory, maxMemGB)
	detailsBox := detailPanelStyle.Render(detailsText)

	// Validation error (if any)
	var errorBox string
	if m.validationErr != "" {
		errorBox = "\n" + errorTitleStyle.Render("⚠ "+m.validationErr) + "\n"
	}

	// Buttons row
	buttonRow := "  " + strings.Join(buttons, "  ")

	// Hints
	hint := formHintStyle.Render("  Tab/↑↓: navigate  ←→: adjust values  Enter: save  Esc: cancel")

	content := titleText + "\n" + tableBox + "\n" + detailsBox + errorBox + "\n" + buttonRow + "\n\n" + hint

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m optionsModel) renderNumericField(input textinput.Model, active bool, maxVal int) string {
	if active {
		left := lipgloss.NewStyle().Foreground(accent).Render("◀ ")
		right := lipgloss.NewStyle().Foreground(accent).Render(" ▶")
		return left + input.View() + right
	}
	return "  " + lipgloss.NewStyle().Foreground(subtle).Render(input.Value()) + "  "
}
