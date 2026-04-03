// view_chat.go - Chat panel model with message viewport and text input
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// chatEntry is a single item in the chat log.
type chatEntry struct {
	role    string // "user", "assistant", "tool-start", "tool-done", "error", "system"
	content string
}

// chatModel is the BubbleTea model for the chat panel.
type chatModel struct {
	width   int
	height  int
	focused bool

	// Chat state
	entries  []chatEntry
	messages []ChatMessage // conversation history for LLM

	// Input
	input textarea.Model

	// Viewport for scrolling messages
	viewport viewport.Model

	// Spinner for agent processing
	spinner  spinner.Model
	thinking bool

	// Infrastructure (set from rootModel)
	llmClient *LLMClient
	mcpClient *MCPClient
	mcpTools  []ToolDef
	program   *tea.Program
	config    LLMConfig

	// MCP state
	mcpReady      bool
	mcpInitFailed bool
	mcpInitErr    string

	// Current VM state (updated from rootModel's table)
	currentVMs []vmData
}

func newChatModel() chatModel {
	ti := textarea.New()
	ti.Placeholder = "Ask about VMs..."
	ti.CharLimit = 500
	ti.ShowLineNumbers = false
	ti.Prompt = ""
	ti.EndOfBufferCharacter = ' '
	ti.SetHeight(3)
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.BlurredStyle.CursorLine = lipgloss.NewStyle()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	vp := viewport.New(40, 10)

	return chatModel{
		input:    ti,
		spinner:  sp,
		viewport: vp,
		entries: []chatEntry{
			{role: "system", content: "Enter to send, Alt+Enter for newline. ? or Tab to switch focus."},
		},
		messages: []ChatMessage{
			{Role: "system", Content: LLMSystemPrompt},
		},
	}
}

func (m chatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m chatModel) Update(msg tea.Msg) (chatModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "enter":
			// Enter sends the message; alt+enter / shift+enter add newlines
			text := strings.TrimSpace(m.input.Value())
			if text == "" || m.thinking {
				return m, nil
			}

			// Check if LLM is configured
			if m.config.APIKey == "" && !isLocalEndpoint(m.config.BaseURL) {
				m.entries = append(m.entries, chatEntry{
					role:    "error",
					content: "No API key configured. Edit ~/.passgo/llm.conf or press L to open settings.",
				})
				m.refreshViewport()
				return m, nil
			}

			// Add user message
			m.entries = append(m.entries, chatEntry{role: "user", content: text})
			m.messages = append(m.messages, ChatMessage{Role: "user", Content: text})
			m.input.Reset()
			m.input.SetHeight(3)
			m.recalcLayout()
			m.thinking = true
			m.refreshViewport()

			// Update system prompt with current VM state before each run
			m.messages[0] = ChatMessage{Role: "system", Content: buildSystemPrompt(m.currentVMs)}

			// Initialize MCP if needed, then run agent
			if !m.mcpReady && !m.mcpInitFailed {
				return m, tea.Batch(m.spinner.Tick, m.initMCPAndRunCmd())
			}
			if m.mcpInitFailed {
				return m, tea.Batch(m.spinner.Tick, m.runAgentWithoutToolsCmd())
			}
			return m, tea.Batch(m.spinner.Tick, m.runAgentCmd())

		case "ctrl+c":
			return m, nil
		}

		// Forward to textarea
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		// Resize textarea height to fit content
		m.resizeInputToContent()

		return m, cmd

	case chatAgentTextMsg:
		m.entries = append(m.entries, chatEntry{
			role:    "assistant",
			content: msg.text,
		})
		m.refreshViewport()
		return m, nil

	case chatToolStartMsg:
		// No separate entry for tool-start; tool-done will show the result
		return m, nil

	case chatToolDoneMsg:
		summary := msg.name
		if msg.err != nil {
			summary += " failed: " + msg.err.Error()
		} else {
			// Show truncated result for context
			result := msg.result
			if len(result) > 60 {
				result = result[:60] + "..."
			}
			summary += " " + result
		}
		m.entries = append(m.entries, chatEntry{
			role:    "tool-done",
			content: summary,
		})
		m.refreshViewport()
		return m, nil

	case chatAgentResultMsg:
		m.thinking = false
		if msg.err != nil {
			m.entries = append(m.entries, chatEntry{
				role:    "error",
				content: msg.err.Error(),
			})
		} else {
			m.entries = append(m.entries, chatEntry{
				role:    "assistant",
				content: msg.response,
			})
			m.messages = append(m.messages, ChatMessage{
				Role:    "assistant",
				Content: msg.response,
			})
		}
		m.refreshViewport()
		return m, nil

	case chatMCPReadyMsg:
		if msg.err != nil {
			m.mcpInitFailed = true
			m.mcpInitErr = msg.err.Error()
			m.entries = append(m.entries, chatEntry{
				role:    "system",
				content: "MCP tools unavailable: " + msg.err.Error(),
			})
		} else {
			m.mcpReady = true
			m.mcpTools = msg.tools
		}
		m.refreshViewport()
		return m, nil

	case chatMCPInitDoneMsg:
		// MCP client initialized in background goroutine — store it
		if msg.err != nil {
			m.mcpInitFailed = true
			m.mcpInitErr = msg.err.Error()
		} else {
			m.mcpClient = msg.client
			m.mcpReady = true
			m.mcpTools = msg.tools
		}
		return m, nil

	case chatMCPDownloadProgressMsg:
		m.entries = append(m.entries, chatEntry{
			role:    "system",
			content: msg.message,
		})
		m.refreshViewport()
		return m, nil

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Forward viewport updates (mouse wheel scroll, etc.)
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// maxChatEntries caps the displayed chat history to prevent unbounded memory growth.
const maxChatEntries = 200

// refreshViewport updates viewport content and scrolls to bottom.
func (m *chatModel) refreshViewport() {
	// Trim old entries if needed
	if len(m.entries) > maxChatEntries {
		m.entries = m.entries[len(m.entries)-maxChatEntries:]
	}
	content := m.renderEntries()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m chatModel) renderEntries() string {
	t := currentTheme()
	var lines []string

	contentWidth := m.width - 4 // padding
	if contentWidth < 10 {
		contentWidth = 10
	}

	for i, e := range m.entries {
		switch e.role {
		case "user":
			label := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("You: ")
			text := lipgloss.NewStyle().Foreground(t.Text).Width(contentWidth - 5).Render(e.content)
			lines = append(lines, label+text)
		case "assistant":
			label := lipgloss.NewStyle().Foreground(t.Running).Bold(true).Render("AI: ")
			text := lipgloss.NewStyle().Foreground(t.Text).Width(contentWidth - 4).Render(e.content)
			lines = append(lines, label+text)
		case "tool-start":
			icon := lipgloss.NewStyle().Foreground(t.Suspended).Render("  > ")
			text := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(e.content)
			lines = append(lines, icon+text)
		case "tool-done":
			icon := lipgloss.NewStyle().Foreground(t.Suspended).Render("  > ")
			text := lipgloss.NewStyle().Foreground(t.TextMuted).Render(e.content)
			lines = append(lines, icon+text)
		case "error":
			label := lipgloss.NewStyle().Foreground(t.Stopped).Bold(true).Render("Error: ")
			text := lipgloss.NewStyle().Foreground(t.Stopped).Width(contentWidth - 7).Render(e.content)
			lines = append(lines, label+text)
		case "system":
			text := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true).Width(contentWidth).Render(e.content)
			lines = append(lines, text)
		}
		// Skip blank line between consecutive tool entries for compact display
		nextIsTool := i+1 < len(m.entries) && (m.entries[i+1].role == "tool-done" || m.entries[i+1].role == "tool-start")
		if !(e.role == "tool-done" && nextIsTool) {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// chatTitleText returns the current title string for the chat panel.
func (m chatModel) chatTitleText() string {
	if m.thinking {
		return m.spinner.View() + " Thinking..."
	}
	if m.mcpReady {
		return "AI Chat (MCP)"
	}
	return "AI Chat"
}

// ViewContent renders the chat content area (viewport + input) without title or outer border.
// Used by the root View for the unified split layout.
func (m chatModel) ViewContent() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	t := currentTheme()

	// Viewport content
	vpContent := m.viewport.View()

	// Input with a visible border box
	inputWidth := m.width - 4 // padding for input border
	if inputWidth < 10 {
		inputWidth = 10
	}
	inputBorderColor := t.Subtle
	if m.focused {
		inputBorderColor = t.Accent
	}
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inputBorderColor).
		Width(inputWidth)
	inputBox := inputStyle.Render(m.input.View())

	return lipgloss.JoinVertical(lipgloss.Left, vpContent, inputBox)
}

func (m chatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	t := currentTheme()

	borderColor := t.Subtle
	if m.focused {
		borderColor = t.Accent
	}

	// Title bar
	titleWidth := m.width - 2
	if titleWidth < 10 {
		titleWidth = 10
	}
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Highlight).
		Background(t.Accent).
		Padding(0, 1).
		Width(titleWidth)

	title := titleStyle.Render(m.chatTitleText())

	// Viewport content
	vpContent := m.viewport.View()

	// Input with a visible border box
	inputWidth := m.width - 6
	if inputWidth < 10 {
		inputWidth = 10
	}
	inputBorderColor := t.Subtle
	if m.focused {
		inputBorderColor = t.Accent
	}
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inputBorderColor).
		Width(inputWidth)
	inputBox := inputStyle.Render(m.input.View())

	content := lipgloss.JoinVertical(lipgloss.Left, title, vpContent, inputBox)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(m.width - 2).
		Height(m.height - 2)

	return border.Render(content)
}

// setSize updates the chat model dimensions and resizes the viewport.
func (m *chatModel) setSize(width, height int) {
	m.width = width
	m.height = height

	inputWidth := width - 6 // padding for input border
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.input.SetWidth(inputWidth)

	m.recalcLayout()
	m.refreshViewport()
}

// resizeInputToContent adjusts textarea height based on content including soft-wrapped lines.
func (m *chatModel) resizeInputToContent() {
	const minInputHeight = 3

	// Calculate visual lines: each hard line may wrap across multiple visual lines
	inputWidth := m.input.Width()
	if inputWidth < 1 {
		inputWidth = 1
	}
	val := m.input.Value()
	visualLines := 0
	if val == "" {
		visualLines = 1
	} else {
		for _, line := range strings.Split(val, "\n") {
			lineLen := len([]rune(line))
			if lineLen == 0 {
				visualLines++
			} else {
				visualLines += (lineLen + inputWidth - 1) / inputWidth
			}
		}
	}

	maxInputHeight := m.height / 3
	if maxInputHeight < minInputHeight {
		maxInputHeight = minInputHeight
	}

	h := visualLines
	if h < minInputHeight {
		h = minInputHeight
	}
	if h > maxInputHeight {
		h = maxInputHeight
	}
	m.input.SetHeight(h)
	m.recalcLayout()
}

// recalcLayout adjusts viewport height based on current input height.
func (m *chatModel) recalcLayout() {
	// input textarea lines + input border box(2)
	inputHeight := m.input.Height() + 2 // textarea content + rounded border
	vpHeight := m.height - inputHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := m.width - 4
	if vpWidth < 10 {
		vpWidth = 10
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
}

// Focus/blur management
func (m *chatModel) Focus() {
	m.focused = true
	m.input.Focus()
}

func (m *chatModel) Blur() {
	m.focused = false
	m.input.Blur()
}

// initMCPAndRunCmd initializes MCP client, then runs the agent.
// All model mutations happen via messages — no direct field writes from the goroutine.
func (m *chatModel) initMCPAndRunCmd() tea.Cmd {
	// Capture values needed by the goroutine (avoid reading m.* during execution)
	program := m.program
	llmClient := m.llmClient
	configMCPBinary := m.config.MCPBinary
	messages := make([]ChatMessage, len(m.messages))
	copy(messages, m.messages)

	return func() tea.Msg {
		// Find or download MCP binary
		binaryPath := findMCPBinary(configMCPBinary)
		if binaryPath == "" {
			var downloadErr error
			binaryPath, downloadErr = downloadMCPBinary(func(msg string) {
				if program != nil {
					program.Send(chatMCPDownloadProgressMsg{message: msg})
				}
			})
			if downloadErr != nil {
				if program != nil {
					program.Send(chatMCPReadyMsg{err: downloadErr})
				}
				return runWithoutTools(llmClient, messages)
			}
		}

		// Start MCP client
		client, err := NewMCPClient(binaryPath)
		if err != nil {
			if program != nil {
				program.Send(chatMCPReadyMsg{err: err})
			}
			return runWithoutTools(llmClient, messages)
		}

		// List tools
		tools, err := client.ListTools()
		if err != nil {
			if program != nil {
				program.Send(chatMCPReadyMsg{err: err})
			}
			return runWithoutTools(llmClient, messages)
		}

		// Notify UI about MCP + store the client via message
		if program != nil {
			program.Send(chatMCPInitDoneMsg{client: client, tools: tools})
			program.Send(chatMCPReadyMsg{tools: tools})
		}

		// Run agent with tools
		result := RunAgent(context.Background(), program, llmClient, client, messages, tools)
		return chatAgentResultMsg{response: result.Response, err: result.Err}
	}
}

// runAgentCmd runs the agent with MCP tools.
func (m *chatModel) runAgentCmd() tea.Cmd {
	// Capture values
	program := m.program
	llmClient := m.llmClient
	mcpClient := m.mcpClient
	tools := m.mcpTools
	messages := make([]ChatMessage, len(m.messages))
	copy(messages, m.messages)

	return func() tea.Msg {
		result := RunAgent(context.Background(), program, llmClient, mcpClient, messages, tools)
		return chatAgentResultMsg{response: result.Response, err: result.Err}
	}
}

// runAgentWithoutToolsCmd runs the agent without tools.
func (m *chatModel) runAgentWithoutToolsCmd() tea.Cmd {
	llmClient := m.llmClient
	messages := make([]ChatMessage, len(m.messages))
	copy(messages, m.messages)

	return func() tea.Msg {
		return runWithoutTools(llmClient, messages)
	}
}

// runWithoutTools calls the LLM with no tools available. Safe to call from goroutines.
func runWithoutTools(llmClient *LLMClient, messages []ChatMessage) chatAgentResultMsg {
	resp, err := llmClient.Chat(context.Background(), messages, nil)
	if err != nil {
		return chatAgentResultMsg{err: err}
	}
	return chatAgentResultMsg{response: resp.Content}
}

// isLocalEndpoint checks if the URL points to a local server (no API key needed).
func isLocalEndpoint(url string) bool {
	return strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")
}

// buildSystemPrompt creates the system prompt with current VM state injected.
func buildSystemPrompt(vms []vmData) string {
	prompt := LLMSystemPrompt + "\n\nCURRENT VM STATE:\n"
	if len(vms) == 0 {
		prompt += "No instances found.\n"
		return prompt
	}
	prompt += fmt.Sprintf("Total instances: %d\n", len(vms))
	for _, vm := range vms {
		info := vm.info
		line := fmt.Sprintf("- %s: state=%s", info.Name, info.State)
		if info.IPv4 != "" && info.IPv4 != "--" {
			line += fmt.Sprintf(", ip=%s", info.IPv4)
		}
		if info.CPUs != "" && info.CPUs != "--" {
			line += fmt.Sprintf(", cpus=%s", info.CPUs)
		}
		if info.DiskUsage != "" && info.DiskUsage != "--" {
			line += fmt.Sprintf(", disk=%s", info.DiskUsage)
		}
		if info.MemoryUsage != "" && info.MemoryUsage != "--" {
			line += fmt.Sprintf(", memory=%s", info.MemoryUsage)
		}
		prompt += line + "\n"
	}
	return prompt
}
