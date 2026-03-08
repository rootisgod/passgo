// mcp_client.go - MCP client: spawn multipass-mcp subprocess, JSON-RPC over stdio
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// MCP timeouts
const (
	mcpCallTimeout = 60 * time.Second  // timeout for individual tool calls
	mcpInitTimeout = 15 * time.Second  // timeout for initialization handshake
	mcpReadLimit   = 10 * 1024 * 1024  // 10MB max response line size
)

// MCPClient manages a multipass-mcp subprocess communicating via JSON-RPC over stdio.
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64
	tools  []ToolDef
	closed atomic.Bool
}

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC error.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpToolsResult is the result of tools/list.
type mcpToolsResult struct {
	Tools []mcpToolInfo `json:"tools"`
}

// mcpToolInfo describes a tool from the MCP server.
type mcpToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// mcpCallToolResult is the result of tools/call.
type mcpCallToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// mcpContent is a content block in a tool call result.
type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewMCPClient spawns the multipass-mcp binary and performs the initialize handshake.
func NewMCPClient(binaryPath string) (*MCPClient, error) {
	cmd := exec.Command(binaryPath) // #nosec G204 -- binary path from user config or auto-download
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("start MCP server: %w", err)
	}

	c := &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReaderSize(stdout, 64*1024), // 64KB buffer
	}

	// Initialize handshake with timeout
	initCtx, initCancel := context.WithTimeout(context.Background(), mcpInitTimeout)
	defer initCancel()

	_, err = c.callWithContext(initCtx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "passgo",
			"version": "1.0.0",
		},
	})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}

	// Send initialized notification (no response expected)
	if err := c.notify("notifications/initialized", nil); err != nil {
		c.Close()
		return nil, fmt.Errorf("MCP initialized notification: %w", err)
	}

	return c, nil
}

// ListTools fetches and caches available tools from the MCP server.
func (c *MCPClient) ListTools() ([]ToolDef, error) {
	if len(c.tools) > 0 {
		return c.tools, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), mcpCallTimeout)
	defer cancel()

	result, err := c.callWithContext(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	var toolsResult mcpToolsResult
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}

	tools := make([]ToolDef, len(toolsResult.Tools))
	for i, t := range toolsResult.Tools {
		tools[i] = ToolDef{
			Type: "function",
			Function: ToolDefFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}

	c.tools = tools
	return tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (string, error) {
	// Apply default timeout if context has none
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, mcpCallTimeout)
		defer cancel()
	}

	params := map[string]interface{}{
		"name": name,
	}
	if arguments != nil {
		params["arguments"] = arguments
	}

	result, err := c.callWithContext(ctx, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("tools/call %s: %w", name, err)
	}

	var callResult mcpCallToolResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("parse tool result: %w", err)
	}

	var texts []string
	for _, content := range callResult.Content {
		if content.Type == "text" && content.Text != "" {
			texts = append(texts, content.Text)
		}
	}

	resultText := ""
	if len(texts) > 0 {
		resultText = texts[0]
		for _, t := range texts[1:] {
			resultText += "\n" + t
		}
	}

	if callResult.IsError {
		return "", fmt.Errorf("tool %s error: %s", name, resultText)
	}

	return resultText, nil
}

// Close terminates the MCP subprocess.
func (c *MCPClient) Close() error {
	if c.closed.Swap(true) {
		return nil // already closed
	}
	c.mu.Lock()
	c.stdin.Close()
	c.mu.Unlock()

	// Wait with timeout — kill if subprocess doesn't exit
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		c.cmd.Process.Kill() // #nosec G104 -- best-effort kill
		return fmt.Errorf("MCP server did not exit, killed")
	}
}

// callWithContext sends a JSON-RPC request and reads the response, respecting context cancellation.
func (c *MCPClient) callWithContext(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("MCP client is closed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Write request followed by newline
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response with context timeout
	type readResult struct {
		data json.RawMessage
		err  error
	}
	ch := make(chan readResult, 1)

	go func() {
		for {
			line, err := c.reader.ReadBytes('\n')
			if err != nil {
				ch <- readResult{err: fmt.Errorf("read response: %w", err)}
				return
			}

			// Reject oversized responses
			if len(line) > mcpReadLimit {
				ch <- readResult{err: fmt.Errorf("response too large (%d bytes)", len(line))}
				return
			}

			var resp jsonRPCResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				// Could be a notification, skip it
				continue
			}

			// Skip notifications (no ID)
			if resp.ID == 0 && resp.Result == nil && resp.Error == nil {
				continue
			}

			if resp.ID == id {
				if resp.Error != nil {
					ch <- readResult{err: fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)}
					return
				}
				ch <- readResult{data: resp.Result}
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("MCP call %s: %w", method, ctx.Err())
	case result := <-ch:
		return result.data, result.err
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (c *MCPClient) notify(method string, params interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("MCP client is closed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	type notification struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}

	req := notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}
