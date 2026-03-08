// view_llm_settings.go - LLM configuration editor form
package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// llmSettingsSavedMsg is sent when settings are saved successfully.
type llmSettingsSavedMsg struct{ config LLMConfig }

type llmSettingsModel struct {
	fields []llmSettingsField
	cursor int
	width  int
	height int
	err    string // validation or save error
}

type llmSettingsField struct {
	label    string
	input    textinput.Model
	masked   bool // for API key
	isSubmit bool
	isCancel bool
}

func newLLMSettingsModel(cfg LLMConfig, width, height int) llmSettingsModel {
	urlInput := textinput.New()
	urlInput.Placeholder = "https://openrouter.ai/api/v1"
	urlInput.SetValue(cfg.BaseURL)
	urlInput.CharLimit = 200
	urlInput.Width = 50
	urlInput.Focus()

	keyInput := textinput.New()
	keyInput.Placeholder = "(not required for local endpoints)"
	keyInput.SetValue(cfg.APIKey)
	keyInput.CharLimit = 200
	keyInput.Width = 50
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.EchoCharacter = '•'

	modelInput := textinput.New()
	modelInput.Placeholder = "deepseek/deepseek-v3.2"
	modelInput.SetValue(cfg.Model)
	modelInput.CharLimit = 100
	modelInput.Width = 50

	fields := []llmSettingsField{
		{label: "Base URL", input: urlInput},
		{label: "API Key", input: keyInput, masked: true},
		{label: "Model", input: modelInput},
		{label: "[ Save ]", isSubmit: true},
		{label: "[ Cancel ]", isCancel: true},
	}

	return llmSettingsModel{
		fields: fields,
		width:  width,
		height: height,
	}
}

func (m llmSettingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m llmSettingsModel) Update(msg tea.Msg) (llmSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
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

		case "enter":
			f := m.fields[m.cursor]
			if f.isCancel {
				return m, func() tea.Msg { return backToTableMsg{} }
			}
			if f.isSubmit {
				return m, m.save()
			}
			// Move to next field
			m.blurCurrent()
			m.cursor = (m.cursor + 1) % len(m.fields)
			m.focusCurrent()
			return m, nil
		}

		// Forward to current text input
		f := &m.fields[m.cursor]
		if !f.isSubmit && !f.isCancel {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return m, cmd
		}
	}

	// Forward tick messages for cursor blink
	f := &m.fields[m.cursor]
	if !f.isSubmit && !f.isCancel {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *llmSettingsModel) blurCurrent() {
	f := &m.fields[m.cursor]
	if !f.isSubmit && !f.isCancel {
		f.input.Blur()
	}
}

func (m *llmSettingsModel) focusCurrent() {
	f := &m.fields[m.cursor]
	if !f.isSubmit && !f.isCancel {
		f.input.Focus()
	}
}

func (m llmSettingsModel) save() tea.Cmd {
	baseURL := strings.TrimSpace(m.fields[0].input.Value())
	apiKey := strings.TrimSpace(m.fields[1].input.Value())
	model := strings.TrimSpace(m.fields[2].input.Value())

	if baseURL == "" {
		baseURL = DefaultLLMBaseURL
	}
	if model == "" {
		model = DefaultLLMModel
	}

	cfg := LLMConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}

	return func() tea.Msg {
		if err := saveLLMConfig(cfg); err != nil {
			return llmSettingsSavedMsg{config: cfg}
		}
		return llmSettingsSavedMsg{config: cfg}
	}
}

func (m llmSettingsModel) View() string {
	titleLabel := " ◆ LLM Settings"
	w := min(m.width-4, 70)
	if w < 40 {
		w = 40
	}
	titleText := titleBarStyle.Render(titleLabel)
	titleVisibleWidth := lipgloss.Width(titleText)
	if w > titleVisibleWidth {
		pad := strings.Repeat(" ", w-titleVisibleWidth)
		titleText += lipgloss.NewStyle().Background(accent).Render(pad)
	}

	// Column widths
	labelW := 12
	valueW := w - labelW - 5
	if valueW < 20 {
		valueW = 20
	}

	divStyle := lipgloss.NewStyle().Foreground(dimmed)
	div := divStyle.Render("│")

	// Header
	headerLabel := tableHeaderStyle.Width(labelW).Render("Setting")
	headerValue := tableHeaderStyle.Width(valueW).Render("Value")
	header := "  " + headerLabel + div + headerValue

	// Separator
	sep := "  " + divStyle.Render(strings.Repeat("─", labelW)) +
		divStyle.Render("┼") +
		divStyle.Render(strings.Repeat("─", valueW))

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

		prefix := "  "
		if active {
			prefix = tableCursorStyle.Render("▎ ")
		}

		labelStyle := formLabelStyle.Width(labelW)
		if active {
			labelStyle = formActiveLabelStyle.Width(labelW)
		}
		label := labelStyle.Render(f.label)

		var value string
		if active {
			value = f.input.View()
		} else {
			display := f.input.Value()
			if f.masked && display != "" {
				display = strings.Repeat("•", min(len(display), 20))
			}
			if display == "" {
				display = lipgloss.NewStyle().Foreground(subtle).Italic(true).Render(f.input.Placeholder)
			} else {
				display = lipgloss.NewStyle().Foreground(subtle).Render(display)
			}
			value = display
		}

		rows = append(rows, prefix+label+div+value)
	}

	tableContent := header + "\n" + sep + "\n" + strings.Join(rows, "\n")
	tableBox := tableBorderStyle.Width(w + 2).Render(tableContent)

	buttonRow := "  " + strings.Join(buttons, "  ")

	presets := formHintStyle.Render("  Presets: OpenRouter → openrouter.ai/api/v1 | Ollama → localhost:11434/v1")
	hint := formHintStyle.Render("  Tab/↑↓: navigate  Enter: submit  Esc: cancel")

	content := titleText + "\n" + tableBox + "\n" + buttonRow + "\n\n" + presets + "\n" + hint

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
