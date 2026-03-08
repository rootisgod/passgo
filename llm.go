// llm.go - OpenAI-compatible LLM client and shared types
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxLLMResponseBytes limits the LLM response body to prevent OOM from malicious responses.
const maxLLMResponseBytes = 10 * 1024 * 1024 // 10MB

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef defines a tool available for the LLM to call.
type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolDefFunction `json:"function"`
}

// ToolDefFunction describes a function tool.
type ToolDefFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// LLMClient is an OpenAI-compatible chat completions client.
type LLMClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewLLMClient creates a new LLM client from config.
func NewLLMClient(cfg LLMConfig) *LLMClient {
	return &LLMClient{
		BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// chatRequest is the request body for the chat completions endpoint.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []ToolDef     `json:"tools,omitempty"`
}

// chatResponse is the response from the chat completions endpoint.
type chatResponse struct {
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends a chat completions request and returns the assistant's response.
func (c *LLMClient) Chat(ctx context.Context, messages []ChatMessage, tools []ToolDef) (ChatMessage, error) {
	reqBody := chatRequest{
		Model:    c.Model,
		Messages: messages,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ChatMessage{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	// Limit response body size to prevent OOM
	limitedReader := io.LimitReader(resp.Body, maxLLMResponseBytes)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ChatMessage{}, fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return ChatMessage{}, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return ChatMessage{}, fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return ChatMessage{}, fmt.Errorf("LLM returned no choices")
	}

	msg := chatResp.Choices[0].Message
	msg.Role = "assistant"
	return msg, nil
}

// truncate shortens a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
