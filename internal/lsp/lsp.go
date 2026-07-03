// Package lsp provides a basic Language Server Protocol client for
// code diagnostics and navigation.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client communicates with a language server.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *json.Decoder
	mu     sync.Mutex
	nextID int
}

// Diagnostic represents a single diagnostic (error/warning) from the LSP.
type Diagnostic struct {
	Range    Range    `json:"range"`
	Severity int      `json:"severity"`
	Message  string   `json:"message"`
	Source   string   `json:"source,omitempty"`
}

// Range represents a text range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Symbol represents a code symbol (function, type, variable, etc.).
type Symbol struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	Location       Location `json:"location"`
	ContainerName  string `json:"containerName,omitempty"`
}

// Location represents a location in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// DiagnosticSeverity levels.
const (
	SeverityError   = 1
	SeverityWarning = 2
	SeverityInfo    = 3
	SeverityHint    = 4
)

// SymbolKind values.
const (
	SymbolKindFile        = 1
	SymbolKindModule      = 2
	SymbolKindNamespace   = 3
	SymbolKindPackage     = 4
	SymbolKindClass       = 5
	SymbolKindMethod      = 6
	SymbolKindProperty    = 7
	SymbolKindField       = 8
	SymbolKindConstructor = 9
	SymbolKindEnum        = 10
	SymbolKindInterface   = 11
	SymbolKindFunction    = 12
	SymbolKindVariable    = 13
	SymbolKindConstant    = 14
	SymbolKindString      = 15
	SymbolKindNumber      = 16
	SymbolKindBoolean     = 17
	SymbolKindArray       = 18
)

// NewClient creates a new LSP client that connects to the given command.
func NewClient(command string, args []string) (*Client, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: start: %w", err)
	}

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: json.NewDecoder(stdout),
	}, nil
}

// Initialize performs the LSP initialize handshake.
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := map[string]any{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"diagnostic": map[string]any{},
			},
		},
	}
	_, err := c.call(ctx, "initialize", params)
	if err != nil {
		return err
	}
	return c.notify("initialized", nil)
}

// GetDiagnostics requests diagnostics for a file.
func (c *Client) GetDiagnostics(ctx context.Context, uri string) ([]Diagnostic, error) {
	// Open the file first
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	}
	_, err := c.call(ctx, "textDocument/didOpen", params)
	if err != nil {
		return nil, err
	}

	// In a real implementation, we'd wait for diagnostic notifications.
	// For now, return an empty slice.
	return nil, nil
}

// GetSymbols requests document symbols for a file.
func (c *Client) GetSymbols(ctx context.Context, uri string) ([]Symbol, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	}
	result, err := c.call(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []Symbol
	if data, ok := result.([]any); ok {
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				symbol := Symbol{
					Name: getString(m, "name"),
					Kind: getInt(m, "kind"),
				}
				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols, nil
}

// Shutdown shuts down the language server.
func (c *Client) Shutdown(ctx context.Context) error {
	c.call(ctx, "shutdown", nil)
	c.notify("exit", nil)
	c.stdin.Close()
	return c.cmd.Process.Kill()
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

func (c *Client) call(ctx context.Context, method string, params any) (any, error) {
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

	// Write with Content-Length header
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	_, err = c.stdin.Write([]byte(header))
	if err == nil {
		_, err = c.stdin.Write(data)
	}
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	// Read response
	var resp jsonrpcResponse
	if err := c.stdout.Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("lsp error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	var result any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, nil
	}
	return result, nil
}

func (c *Client) notify(method string, params any) error {
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

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
