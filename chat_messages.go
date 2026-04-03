// chat_messages.go - Chat-specific tea.Msg types and tea.Cmd factories
package main

// chatToolStartMsg is sent when a tool call begins executing.
type chatToolStartMsg struct {
	name string
	args string
}

// chatToolDoneMsg is sent when a tool call completes.
type chatToolDoneMsg struct {
	name   string
	result string
	err    error
}

// chatAgentTextMsg carries intermediate assistant text emitted alongside tool calls.
type chatAgentTextMsg struct {
	text string
}

// chatAgentResultMsg carries the final agent response.
type chatAgentResultMsg struct {
	response string
	err      error
}

// chatMCPReadyMsg is sent when MCP client is initialized and tools are available.
type chatMCPReadyMsg struct {
	tools []ToolDef
	err   error
}

// chatMCPInitDoneMsg carries the MCP client and tools back to the model safely.
type chatMCPInitDoneMsg struct {
	client *MCPClient
	tools  []ToolDef
	err    error
}

// chatMCPDownloadProgressMsg reports MCP binary download progress.
type chatMCPDownloadProgressMsg struct {
	message string
}
