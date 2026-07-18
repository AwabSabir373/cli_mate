package tools

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"cli_mate/internal/providers/contracts"
)

type bufferWriteCloser struct{ bytes.Buffer }

func (b *bufferWriteCloser) Close() error { return nil }

func TestMCPClientCallUsesNewlineDelimitedJSONRPC(t *testing.T) {
	input := &bufferWriteCloser{}
	client := NewMCPClient("unused", nil)
	client.stdin = input
	client.stdout = bufio.NewReader(strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n"))

	result, err := client.call(context.Background(), "ping", nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	request := input.String()
	if strings.Contains(request, "Content-Length") || !strings.HasSuffix(request, "\n") {
		t.Fatalf("request is not standard MCP stdio framing: %q", request)
	}
}

func TestMCPClientSkipsNotificationBeforeResponse(t *testing.T) {
	client := NewMCPClient("unused", nil)
	client.stdin = &bufferWriteCloser{}
	client.stdout = bufio.NewReader(strings.NewReader(
		"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\"}\n" +
			"{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n",
	))
	if _, err := client.call(context.Background(), "tools/list", nil); err != nil {
		t.Fatalf("call with interleaved notification: %v", err)
	}
}

func TestMCPClientCallStopsWhenContextExpires(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	client := NewMCPClient("unused", nil)
	client.stdin = &bufferWriteCloser{}
	client.stdout = bufio.NewReader(reader)
	client.stdoutCloser = reader

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := client.call(ctx, "tools/call", map[string]any{"name": "stuck"})
	if err == nil || !strings.Contains(err.Error(), "timed out or was cancelled") {
		t.Fatalf("expected contextual MCP timeout, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelled MCP call remained blocked for %s", elapsed)
	}
}

func TestMCPRemoteToolsPreserveDiscoveredSchema(t *testing.T) {
	client := NewMCPClient("unused", nil)
	client.tools = []contracts.ToolDefinition{{
		Name:        "search_code",
		Description: "compact search",
		Schema:      map[string]any{"type": "object"},
	}}

	remote := client.RemoteTools()
	if len(remote) != 1 || remote[0].Name() != "search_code" {
		t.Fatalf("unexpected remote tools: %#v", remote)
	}
	definition := remote[0].Definition()
	if definition.Description != "compact search" || definition.Schema["type"] != "object" {
		t.Fatalf("discovered MCP schema was not preserved: %#v", definition)
	}
}
