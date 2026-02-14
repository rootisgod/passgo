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
			}
			return m, nil

		case "right":
			f := &m.fields[m.cursor]
			if f.isSelect && f.optionIdx < len(f.options)-1 {
				f.optionIdx++
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

	cloudInitIdx := m.fields[5].optionIdx
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
		}
	}
}

func (m advCreateModel) View() string {
	title := formTitleStyle.Render("Create New Instance")

	var lines []string
	for i, f := range m.fields {
		active := i == m.cursor

		if f.isSubmit || f.isCancel {
			style := formButtonStyle
			if active {
				style = formActiveButtonStyle
			}
			lines = append(lines, style.Render(f.label))
			continue
		}

		label := formLabelStyle.Render(f.label + ":")
		if active {
			label = formActiveLabelStyle.Render(f.label + ":")
		}

		var value string
		if f.isSelect {
			// Show current option with arrows
			opt := f.options[f.optionIdx]
			left := "  "
			right := "  "
			if f.optionIdx > 0 {
				left = "◀ "
			}
			if f.optionIdx < len(f.options)-1 {
				right = " ▶"
			}
			if active {
				value = formActiveButtonStyle.Render(left + opt + right)
			} else {
				value = formValueStyle.Render("  " + opt + "  ")
			}
		} else {
			if active {
				value = f.input.View()
			} else {
				value = formValueStyle.Render(f.input.Value())
			}
		}

		lines = append(lines, fmt.Sprintf("  %s  %s", lipgloss.NewStyle().Width(16).Render(label), value))
	}

	hint := "\n" + formHintStyle.Render("Tab/↑↓: navigate  ←→: cycle options  Enter: submit  Esc: cancel")

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n" + hint
	box := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
