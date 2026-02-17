// view_create.go - Advanced VM creation form with multiple fields
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// advCreateMsg is sent when the advanced create form is submitted.
type advCreateMsg struct {
	name          string
	release       string
	cpus          int
	memoryMB      int
	diskGB        int
	cloudInitFile string
	networkName   string // "" = NAT, "bridged" = --bridged, else --network <name>
}

type advCreateModel struct {
	fields   []advField
	cursor   int // which field is focused
	width    int
	height   int
	releases []string
	// Cloud-init
	cloudInitOptions []string // display labels
	cloudInitPaths   []string // actual file paths (aligned with options)
	cleanupDirs      []string
	// Network
	networkOptions []string // display labels
	networkNames   []string // actual names for --network or "bridged" (aligned with options)
}

type advField struct {
	label       string
	input       textinput.Model
	isSelect    bool
	options     []string
	optionIdx   int
	isNumeric   bool
	isSubmit    bool
	isCancel    bool
	placeholder string
}

func newAdvCreateModel(width, height int) advCreateModel {
	// Collect cloud-init templates
	templateOptions, cleanupDirs, _ := GetAllCloudInitTemplateOptions()
	cloudInitLabels := []string{"None"}
	cloudInitPaths := []string{""}
	for _, opt := range templateOptions {
		cloudInitLabels = append(cloudInitLabels, opt.Label)
		cloudInitPaths = append(cloudInitPaths, opt.Path)
	}

	// Build network options from multipass networks (cross-platform)
	networkOptions := []string{"Default (NAT)"}
	networkNames := []string{""}
	if nets, err := ListNetworks(); err == nil && len(nets) > 0 {
		for _, n := range nets {
			label := fmt.Sprintf("Bridged: %s (%s)", n.Name, n.Description)
			if len(label) > 50 {
				label = truncateToRunes(label, 47) + "..."
			}
			networkOptions = append(networkOptions, label)
			networkNames = append(networkNames, n.Name)
		}
	} else {
		// Fallback when multipass networks unsupported (e.g. Linux LXD)
		networkOptions = append(networkOptions, "Bridged (default)")
		networkNames = append(networkNames, "bridged")
	}

	nameInput := textinput.New()
	nameInput.Placeholder = "my-vm"
	nameInput.Focus()
	nameInput.CharLimit = 40

	cpuInput := textinput.New()
	cpuInput.SetValue(fmt.Sprintf("%d", DefaultCPUCores))
	cpuInput.CharLimit = 4

	ramInput := textinput.New()
	ramInput.SetValue(fmt.Sprintf("%d", DefaultRAMMB))
	ramInput.CharLimit = 8

	diskInput := textinput.New()
	diskInput.SetValue(fmt.Sprintf("%d", DefaultDiskGB))
	diskInput.CharLimit = 6

	fields := []advField{
		{label: "Instance Name", input: nameInput},
		{label: "Release", isSelect: true, options: UbuntuReleases, optionIdx: DefaultReleaseIndex},
		{label: "CPU Cores", input: cpuInput, isNumeric: true},
		{label: "RAM (MB)", input: ramInput, isNumeric: true},
		{label: "Disk (GB)", input: diskInput, isNumeric: true},
		{label: "Network", isSelect: true, options: networkOptions, optionIdx: 0},
		{label: "Cloud-init", isSelect: true, options: cloudInitLabels, optionIdx: 0},
		{label: "[ Create ]", isSubmit: true},
		{label: "[ Cancel ]", isCancel: true},
	}

	return advCreateModel{
		fields:           fields,
		width:            width,
		height:           height,
		releases:         UbuntuReleases,
		cloudInitOptions: cloudInitLabels,
		cloudInitPaths:   cloudInitPaths,
		cleanupDirs:      cleanupDirs,
		networkOptions:   networkOptions,
		networkNames:     networkNames,
	}
}

func (m advCreateModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m advCreateModel) Update(msg tea.Msg) (advCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			CleanupTempDirs(m.cleanupDirs)
			return m, func() tea.Msg { return backToTableMsg{} }

		case "tab", "down":
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
			return m, nil

		case "shift+tab", "up":
			m.blurCurrent()
			m.cursor = (m.cursor - 1 + len(m.fields)) % len(m.fields)
			m.focusCurrent()
			return m, nil

		case "left":
			f := &m.fields[m.cursor]
			if f.isSelect && f.optionIdx > 0 {
				f.optionIdx--
			} else if f.isNumeric {
				if v, err := strconv.Atoi(f.input.Value()); err == nil {
					if vals := m.niceValues(m.cursor); vals != nil {
						f.input.SetValue(fmt.Sprintf("%d", snapPrev(v, vals)))
					}
				}
			}
			return m, nil

		case "right":
			f := &m.fields[m.cursor]
			if f.isSelect && f.optionIdx < len(f.options)-1 {
				f.optionIdx++
			} else if f.isNumeric {
				if v, err := strconv.Atoi(f.input.Value()); err == nil {
					if vals := m.niceValues(m.cursor); vals != nil {
						f.input.SetValue(fmt.Sprintf("%d", snapNext(v, vals)))
					}
				}
			}
			return m, nil

		case "enter":
			f := m.fields[m.cursor]
			if f.isCancel {
				CleanupTempDirs(m.cleanupDirs)
				return m, func() tea.Msg { return backToTableMsg{} }
			}
			if f.isSubmit {
				return m, m.submit()
			}
			// Move to next field
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
			return m, nil
		}

		// Text input for current field
		f := &m.fields[m.cursor]
		if !f.isSelect && !f.isSubmit && !f.isCancel {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return m, cmd
		}
	}

	// Pass tick messages to focused textinput for cursor blink
	f := &m.fields[m.cursor]
	if !f.isSelect && !f.isSubmit && !f.isCancel {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *advCreateModel) blurCurrent() {
	f := &m.fields[m.cursor]
	if !f.isSelect && !f.isSubmit && !f.isCancel {
		f.input.Blur()
	}
}

func (m *advCreateModel) focusCurrent() {
	f := &m.fields[m.cursor]
	if !f.isSelect && !f.isSubmit && !f.isCancel {
		f.input.Focus()
	}
}

// Predefined "nice" values for numeric fields.
var (
	niceRAMValues  = []int{256, 512, 768, 1024, 1536, 2048, 3072, 4096, 6144, 8192, 10240, 12288, 16384, 24576, 32768, 49152, 65536}
	niceDiskValues = []int{4, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256, 384, 512}
	niceCPUValues  = []int{1, 2, 4, 6, 8, 12, 16, 24, 32, 48, 64}
)

// snapNext returns the next value in the list above currentVal, or the last value.
func snapNext(currentVal int, values []int) int {
	for _, v := range values {
		if v > currentVal {
			return v
		}
	}
	return values[len(values)-1]
}

// snapPrev returns the previous value in the list below currentVal, or the first value.
func snapPrev(currentVal int, values []int) int {
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] < currentVal {
			return values[i]
		}
	}
	return values[0]
}

// niceValues returns the appropriate value list for a field.
func (m advCreateModel) niceValues(fieldIdx int) []int {
	switch m.fields[fieldIdx].label {
	case "RAM (MB)":
		return niceRAMValues
	case "Disk (GB)":
		return niceDiskValues
	case "CPU Cores":
		return niceCPUValues
	default:
		return nil
	}
}

func (m advCreateModel) submit() tea.Cmd {
	name := m.fields[0].input.Value()
	if name == "" {
		return nil // TODO: show validation error
	}

	release := m.fields[1].options[m.fields[1].optionIdx]

	cpus, err := strconv.Atoi(m.fields[2].input.Value())
	if err != nil || cpus < MinCPUCores {
		cpus = DefaultCPUCores
	}

	ram, err := strconv.Atoi(m.fields[3].input.Value())
	if err != nil || ram < MinRAMMB {
		ram = DefaultRAMMB
	}

	disk, err := strconv.Atoi(m.fields[4].input.Value())
	if err != nil || disk < MinDiskGB {
		disk = DefaultDiskGB
	}

	networkIdx := m.fields[5].optionIdx
	networkName := ""
	if networkIdx > 0 && networkIdx < len(m.networkNames) {
		networkName = m.networkNames[networkIdx]
	}

	cloudInitIdx := m.fields[6].optionIdx
	cloudInitFile := ""
	if cloudInitIdx > 0 && cloudInitIdx < len(m.cloudInitPaths) {
		cloudInitFile = m.cloudInitPaths[cloudInitIdx]
	}

	CleanupTempDirs(m.cleanupDirs)

	return func() tea.Msg {
		return advCreateMsg{
			name:          name,
			release:       release,
			cpus:          cpus,
			memoryMB:      ram,
			diskGB:        disk,
			cloudInitFile: cloudInitFile,
			networkName:   networkName,
		}
	}
}

func (m advCreateModel) View() string {
	// Title bar styled like the main table
	titleLabel := " ◆ Create New Instance"
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
	labelW := 16
	valueW := w - labelW - 5 // 5 = prefix(3) + div(1) + padding(1)
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
	for i, f := range m.fields {
		active := i == m.cursor

		if f.isSubmit || f.isCancel {
			style := formButtonStyle
			if active {
				style = formActiveButtonStyle
			}
			buttons = append(buttons, style.Render(f.label))
			continue
		}

		// Selection prefix
		prefix := "  "
		if active {
			prefix = tableCursorStyle.Render("▎ ")
		}

		// Label
		labelStyle := formLabelStyle.Width(labelW)
		if active {
			labelStyle = formActiveLabelStyle.Width(labelW)
		}
		label := labelStyle.Render(f.label)

		// Value
		var value string
		if f.isSelect {
			opt := f.options[f.optionIdx]
			left := "  "
			right := "  "
			if f.optionIdx > 0 {
				left = lipgloss.NewStyle().Foreground(accent).Render("◀ ")
			}
			if f.optionIdx < len(f.options)-1 {
				right = lipgloss.NewStyle().Foreground(accent).Render(" ▶")
			}
			if active {
				value = left + formValueStyle.Render(opt) + right
			} else {
				value = "  " + lipgloss.NewStyle().Foreground(subtle).Render(opt) + "  "
			}
		} else if f.isNumeric {
			if active {
				left := lipgloss.NewStyle().Foreground(accent).Render("◀ ")
				right := lipgloss.NewStyle().Foreground(accent).Render(" ▶")
				value = left + f.input.View() + right
			} else {
				value = "  " + lipgloss.NewStyle().Foreground(subtle).Render(f.input.Value()) + "  "
			}
		} else {
			if active {
				value = f.input.View()
			} else {
				value = lipgloss.NewStyle().Foreground(subtle).Render(f.input.Value())
			}
		}

		rows = append(rows, prefix+label+div+value)
	}

	// Build table inside a border box
	tableContent := header + "\n" + sep + "\n" + strings.Join(rows, "\n")
	tableBox := tableBorderStyle.Width(w + 2).Render(tableContent)

	// Buttons row
	buttonRow := "  " + strings.Join(buttons, "  ")

	// Hints
	hint := formHintStyle.Render("  Tab/↑↓: navigate  ←→: adjust values  Enter: submit  Esc: cancel")

	content := titleText + "\n" + tableBox + "\n" + buttonRow + "\n\n" + hint

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
