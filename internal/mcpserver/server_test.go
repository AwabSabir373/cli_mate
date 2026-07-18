package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServerUsesNewlineDelimitedJSONRPCAndPreservesStringID(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":"req-1","method":"initialize","params":{}}` + "\n")
	var output bytes.Buffer
	server := newServer(input, &output)

	req, err := server.readRequest()
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	server.handleRequest(req)

	if strings.Contains(output.String(), "Content-Length") {
		t.Fatalf("MCP stdio output used non-standard LSP framing: %q", output.String())
	}
	if !strings.HasSuffix(output.String(), "\n") {
		t.Fatalf("MCP response is not newline delimited: %q", output.String())
	}
	var response struct {
		ID     string         `json:"id"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != "req-1" {
		t.Fatalf("string request ID was not preserved: %q", response.ID)
	}
	if response.Result["protocolVersion"] != protocolVersion {
		t.Fatalf("unexpected protocol version: %#v", response.Result)
	}
}

func TestServerDrainsQueuedRequestBeforeEOF(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":"last","method":"ping"}` + "\n")
	var output bytes.Buffer
	server := newServer(input, &output)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	if !strings.Contains(output.String(), `"id":"last"`) || !strings.Contains(output.String(), `"result":{}`) {
		t.Fatalf("final queued request was lost at EOF: %q", output.String())
	}
}

func TestUnknownToolReturnsInvalidParams(t *testing.T) {
	var output bytes.Buffer
	server := newServer(strings.NewReader(""), &output)
	rawID := json.RawMessage(`7`)
	server.handleRequest(&jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      &rawID,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"missing","arguments":{}}`),
	})

	var response struct {
		Error *jsonrpcError `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != -32602 {
		t.Fatalf("expected invalid params error, got %#v", response.Error)
	}
}

func TestToolHandlerPanicBecomesToolError(t *testing.T) {
	var output bytes.Buffer
	server := newServer(strings.NewReader(""), &output)
	server.RegisterTool("panic", func(_ context.Context, _ map[string]any) (any, error) {
		panic("boom")
	})
	rawID := json.RawMessage(`9`)
	server.handleRequest(&jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      &rawID,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"panic","arguments":{}}`),
	})
	if !strings.Contains(output.String(), `"isError":true`) || !strings.Contains(output.String(), "tool handler panic") {
		t.Fatalf("panic was not isolated as a tool error: %s", output.String())
	}
}
