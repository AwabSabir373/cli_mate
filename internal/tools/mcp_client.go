package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"cli_mate/internal/providers/contracts"
)

const defaultMCPCallTimeout = 30 * time.Second

// MCPClient implements the Model Context Protocol over stdio.
type MCPClient struct {
	endpoint     string
	args         []string
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       *bufio.Reader
	stdoutCloser io.ReadCloser
	stderr       bytes.Buffer
	writeMu      sync.Mutex
	callMu       sync.Mutex
	nextID       int
	tools        []contracts.ToolDefinition
}

func NewMCPClient(endpoint string, args []string) *MCPClient {
	return &MCPClient{endpoint: endpoint, args: args}
}

func (c *MCPClient) Name() string {
	return "mcp"
}

func (c *MCPClient) Definition() Definition {
	return Definition{
		Name:        "mcp",
		Description: "Execute a tool on a connected MCP server",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"name", "arguments"},
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "MCP tool name"},
				"arguments": map[string]any{"type": "object", "description": "Tool arguments"},
			},
		},
	}
}

// Connect starts the MCP server process and performs the initialize handshake.
func (c *MCPClient) Connect(ctx context.Context) error {
	c.cmd = exec.CommandContext(ctx, c.endpoint, c.args...)
	// MCP servers use stdout for JSON-RPC and may use stderr for diagnostics.
	// Never attach child stderr to the alternate-screen TUI because it corrupts
	// the rendered frame; retain it so failures can include useful context.
	c.cmd.Stderr = &c.stderr

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdout pipe: %w", err)
	}
	c.stdout = bufio.NewReader(stdout)
	c.stdoutCloser = stdout

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp start: %w", err)
	}
	c.tools = nil

	// Initialize handshake
	initResp, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "cli_mate",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	_ = initResp

	// Send initialized notification
	if err := c.notify("notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp initialized notification: %w", err)
	}

	// List tools
	toolsResp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("mcp tools/list: %w", err)
	}

	if tools, ok := toolsResp["tools"].([]any); ok {
		for _, t := range tools {
			tool, ok := t.(map[string]any)
			if !ok {
				continue
			}
			name, _ := tool["name"].(string)
			desc, _ := tool["description"].(string)
			schema, _ := tool["inputSchema"].(map[string]any)
			if name != "" {
				c.tools = append(c.tools, contracts.ToolDefinition{
					Name:        name,
					Description: desc,
					Schema:      schema,
				})
			}
		}
	}

	return nil
}

// Tools returns the tool definitions discovered from the MCP server.
func (c *MCPClient) Tools() []contracts.ToolDefinition {
	return append([]contracts.ToolDefinition(nil), c.tools...)
}

// RemoteTools adapts discovered MCP tools to the agent's native Tool contract.
func (c *MCPClient) RemoteTools() []Tool {
	result := make([]Tool, 0, len(c.tools))
	for _, definition := range c.tools {
		result = append(result, &mcpRemoteTool{client: c, definition: definition})
	}
	return result
}

type mcpRemoteTool struct {
	client     *MCPClient
	definition contracts.ToolDefinition
}

func (t *mcpRemoteTool) Name() string { return t.definition.Name }

func (t *mcpRemoteTool) Definition() Definition {
	return Definition{
		Name:        t.definition.Name,
		Description: t.definition.Description,
		Schema:      t.definition.Schema,
	}
}

func (t *mcpRemoteTool) Execute(ctx context.Context, call Call) (Result, error) {
	call.Name = t.definition.Name
	return t.client.Execute(ctx, call)
}

// Execute calls a tool on the MCP server.
func (c *MCPClient) Execute(ctx context.Context, call Call) (Result, error) {
	resp, err := c.call(ctx, "tools/call", map[string]any{
		"name":      call.Name,
		"arguments": call.Argument,
	})
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if isError, _ := resp["isError"].(bool); isError {
		message := extractMCPText(resp)
		if message == "" {
			message = "MCP tool returned an error"
		}
		return Result{Error: message}, fmt.Errorf("%s", message)
	}

	if text := extractMCPText(resp); text != "" {
		return Result{Content: text}, nil
	}

	data, _ := json.Marshal(resp)
	return Result{Content: string(data)}, nil
}

func extractMCPText(resp map[string]any) string {
	content, _ := resp["content"].([]any)
	var b strings.Builder
	for _, item := range content {
		block, _ := item.(map[string]any)
		if text, ok := block["text"].(string); ok {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(text)
		}
	}
	return b.String()
}

// Close shuts down the MCP server process.
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.stdoutCloser != nil {
		_ = c.stdoutCloser.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *MCPClient) call(ctx context.Context, method string, params any) (map[string]any, error) {
	c.callMu.Lock()
	defer c.callMu.Unlock()

	c.nextID++
	id := c.nextID

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	err = c.writeMessage(data)
	if err != nil {
		return nil, fmt.Errorf("mcp write: %w", err)
	}

	callCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, defaultMCPCallTimeout)
	}
	defer cancel()

	// A pipe read cannot observe context cancellation by itself. Read on a
	// helper goroutine and close the transport when the call expires, which
	// unblocks the read and prevents a stuck MCP server from freezing the run.
	type responseResult struct {
		response *jsonrpcResponse
		err      error
	}
	resultCh := make(chan responseResult, 1)
	go func() {
		resp, readErr := c.readResponse(id)
		resultCh <- responseResult{response: resp, err: readErr}
	}()

	var resp *jsonrpcResponse
	select {
	case result := <-resultCh:
		resp, err = result.response, result.err
	case <-callCtx.Done():
		_ = c.Close()
		return nil, fmt.Errorf("mcp %s timed out or was cancelled: %w", method, callCtx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(c.stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("%w (server stderr: %s)", err, detail)
		}
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp decode result: %w", err)
	}
	return result, nil
}

func (c *MCPClient) notify(method string, params any) error {
	notif := jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	return c.writeMessage(data)
}

func (c *MCPClient) writeMessage(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.stdin.Write(append(data, '\n'))
	return err
}

func (c *MCPClient) readResponse(expectedID int) (*jsonrpcResponse, error) {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("mcp read response: %w", err)
		}
		line = []byte(strings.TrimSpace(string(line)))
		if len(line) == 0 {
			continue
		}
		var envelope struct {
			ID     *int            `json:"id,omitempty"`
			Method string          `json:"method,omitempty"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *jsonrpcError   `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("mcp decode response: %w", err)
		}
		// Notifications may arrive between a request and its response.
		if envelope.ID == nil {
			continue
		}
		if *envelope.ID != expectedID {
			return nil, fmt.Errorf("mcp response id mismatch: got %d, want %d", *envelope.ID, expectedID)
		}
		return &jsonrpcResponse{JSONRPC: "2.0", ID: *envelope.ID, Result: envelope.Result, Error: envelope.Error}, nil
	}
}
