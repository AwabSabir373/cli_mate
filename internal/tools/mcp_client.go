package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"cli_mate/internal/providers/contracts"
)

// MCPClient implements the Model Context Protocol over stdio.
type MCPClient struct {
	endpoint string
	args     []string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	mu       sync.Mutex
	nextID   int
	tools    []contracts.ToolDefinition
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
	c.cmd.Stderr = os.Stderr

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

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp start: %w", err)
	}

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
	return c.tools
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

	// Extract content from response
	if content, ok := resp["content"].([]any); ok {
		var b strings.Builder
		for _, item := range content {
			if block, ok := item.(map[string]any); ok {
				if t, ok := block["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return Result{Content: b.String()}, nil
	}

	data, _ := json.Marshal(resp)
	return Result{Content: string(data)}, nil
}

// Close shuts down the MCP server process.
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
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
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

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

	// Write request with Content-Length header (LSP-style framing)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	_, err = c.stdin.Write([]byte(header))
	if err == nil {
		_, err = c.stdin.Write(data)
	}
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("mcp write: %w", err)
	}

	// Read response
	resp, err := c.readResponse()
	if err != nil {
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

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.stdin.Write([]byte(header))
	if err == nil {
		_, err = c.stdin.Write(data)
	}
	return err
}

func (c *MCPClient) readResponse() (*jsonrpcResponse, error) {
	// Read Content-Length header
	contentLength := -1
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("mcp read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		var length int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &length); err == nil {
			contentLength = length
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("mcp: missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(c.stdout, body)
	if err != nil {
		return nil, fmt.Errorf("mcp read body: %w", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mcp decode response: %w", err)
	}
	return &resp, nil
}
