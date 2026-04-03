// agent.go - Agent loop: iterates LLM calls <-> MCP tool execution
package main

import (
	"context"
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// maxConversationMessages caps the message history sent to the LLM to prevent
// exceeding token limits. Keeps the system prompt + last N messages.
const maxConversationMessages = 50

// AgentResult holds the outcome of an agent run.
type AgentResult struct {
	Response string
	Err      error
}

// RunAgent executes the agent loop: LLM decides tool calls, MCP executes them,
// results feed back until a text-only response. Sends live progress via p.Send().
func RunAgent(ctx context.Context, p *tea.Program, client *LLMClient,
	mcpClient *MCPClient, messages []ChatMessage, tools []ToolDef) AgentResult {

	// Build allowed tool name set for validation
	allowedTools := make(map[string]bool, len(tools))
	for _, t := range tools {
		allowedTools[t.Function.Name] = true
	}

	for i := 0; i < MaxAgentIterations; i++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return AgentResult{Err: fmt.Errorf("cancelled: %w", err)}
		}

		// Trim conversation history to stay within token limits
		trimmed := trimMessages(messages, maxConversationMessages)

		resp, err := client.Chat(ctx, trimmed, tools)
		if err != nil {
			return AgentResult{Err: fmt.Errorf("LLM error: %w", err)}
		}

		// No tool calls — final text response
		if len(resp.ToolCalls) == 0 {
			return AgentResult{Response: resp.Content}
		}

		// Send intermediate assistant text (e.g. "I'll set up the web server...")
		if resp.Content != "" && p != nil {
			p.Send(chatAgentTextMsg{text: resp.Content})
		}

		// Append assistant message with tool calls
		messages = append(messages, resp)

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			// Check context cancellation between tool calls
			if err := ctx.Err(); err != nil {
				return AgentResult{Err: fmt.Errorf("cancelled: %w", err)}
			}

			// Validate tool call has required fields
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("call_%d_%d", i, 0)
			}

			// Validate tool name against known tools
			if !allowedTools[tc.Function.Name] {
				errMsg := fmt.Sprintf("Unknown tool '%s' — not calling it", tc.Function.Name)
				p.Send(chatToolDoneMsg{name: tc.Function.Name, result: errMsg, err: fmt.Errorf("unknown tool")})
				messages = append(messages, ChatMessage{
					Role:       "tool",
					Content:    errMsg,
					ToolCallID: tc.ID,
				})
				continue
			}

			// Notify UI that tool execution is starting
			p.Send(chatToolStartMsg{
				name: tc.Function.Name,
				args: tc.Function.Arguments,
			})

			// Parse arguments
			var args map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					errMsg := fmt.Sprintf("Invalid arguments for %s: %s", tc.Function.Name, err.Error())
					p.Send(chatToolDoneMsg{name: tc.Function.Name, result: errMsg, err: err})
					messages = append(messages, ChatMessage{
						Role:       "tool",
						Content:    errMsg,
						ToolCallID: tc.ID,
					})
					continue
				}
			}

			// Execute tool via MCP
			result, err := mcpClient.CallTool(ctx, tc.Function.Name, args)

			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
			}

			// Notify UI that tool execution completed
			p.Send(chatToolDoneMsg{
				name:   tc.Function.Name,
				result: result,
				err:    err,
			})

			// Append tool result to conversation
			messages = append(messages, ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return AgentResult{Err: fmt.Errorf("agent exceeded maximum iterations (%d)", MaxAgentIterations)}
}

// trimMessages keeps the system prompt (first message) and the most recent messages
// to stay within a reasonable token budget.
func trimMessages(messages []ChatMessage, maxMessages int) []ChatMessage {
	if len(messages) <= maxMessages {
		return messages
	}
	// Keep system prompt + tail
	tail := messages[len(messages)-(maxMessages-1):]
	result := make([]ChatMessage, 0, maxMessages)
	result = append(result, messages[0]) // system prompt
	result = append(result, tail...)
	return result
}

// runAgentCmd creates a tea.Cmd that runs the agent loop in a goroutine.
func runAgentCmd(ctx context.Context, p *tea.Program, client *LLMClient,
	mcpClient *MCPClient, messages []ChatMessage, tools []ToolDef) tea.Cmd {
	return func() tea.Msg {
		result := RunAgent(ctx, p, client, mcpClient, messages, tools)
		return chatAgentResultMsg{
			response: result.Response,
			err:      result.Err,
		}
	}
}
