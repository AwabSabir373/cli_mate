package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const protocolVersion = "2024-11-05"

// Server handles newline-delimited MCP JSON-RPC 2.0 over stdio.
type Server struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
	tools  map[string]ToolHandler

	activeReqs   map[string]context.CancelFunc
	activeReqsMu sync.Mutex
	workQueue    chan *jsonrpcRequest
	workerWG     sync.WaitGroup
}

type ToolHandler func(ctx context.Context, params map[string]any) (any, error)

func NewServer() *Server { return newServer(os.Stdin, os.Stdout) }

func newServer(reader io.Reader, writer io.Writer) *Server {
	s := &Server{
		reader:     bufio.NewReader(reader),
		writer:     writer,
		tools:      make(map[string]ToolHandler),
		activeReqs: make(map[string]context.CancelFunc),
		workQueue:  make(chan *jsonrpcRequest, 100),
	}
	for i := 0; i < 10; i++ {
		s.workerWG.Add(1)
		go s.worker()
	}
	return s
}

func (s *Server) RegisterTool(name string, handler ToolHandler) { s.tools[name] = handler }

func (s *Server) Start() error {
	for {
		req, err := s.readRequest()
		if err != nil {
			if err == io.EOF {
				close(s.workQueue)
				s.workerWG.Wait()
				return nil
			}
			s.writeResponse(nil, nil, &jsonrpcError{Code: -32700, Message: err.Error()})
			continue
		}
		if req.ID == nil {
			s.handleNotification(req)
			continue
		}
		s.workQueue <- req
	}
}

func (s *Server) worker() {
	defer s.workerWG.Done()
	for req := range s.workQueue {
		s.handleRequest(req)
	}
}

type jsonrpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *jsonrpcError    `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) readRequest() (*jsonrpcRequest, error) {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil && len(line) == 0 {
			return nil, err
		}
		line = []byte(strings.TrimSpace(string(line)))
		if len(line) == 0 {
			if err != nil {
				return nil, err
			}
			continue
		}
		var req jsonrpcRequest
		if decodeErr := json.Unmarshal(line, &req); decodeErr != nil {
			return nil, fmt.Errorf("decode JSON-RPC message: %w", decodeErr)
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			return nil, fmt.Errorf("invalid JSON-RPC request")
		}
		return &req, nil
	}
}

func requestKey(id json.RawMessage) string { return string(id) }

func (s *Server) handleNotification(req *jsonrpcRequest) {
	if req.Method != "notifications/cancelled" {
		return
	}
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(req.Params, &params) == nil {
		s.cancelRequest(requestKey(params.RequestID))
	}
}

func (s *Server) cancelRequest(key string) {
	s.activeReqsMu.Lock()
	cancel := s.activeReqs[key]
	delete(s.activeReqs, key)
	s.activeReqsMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) handleRequest(req *jsonrpcRequest) {
	key := requestKey(*req.ID)
	ctx, cancel := context.WithCancel(context.Background())
	s.activeReqsMu.Lock()
	s.activeReqs[key] = cancel
	s.activeReqsMu.Unlock()
	defer s.cancelRequest(key)

	switch req.Method {
	case "initialize":
		s.writeResponse(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "cli_mcp", "version": "1.2.0"},
		}, nil)
	case "ping":
		s.writeResponse(req.ID, map[string]any{}, nil)
	case "tools/list":
		s.writeResponse(req.ID, map[string]any{"tools": GetToolDefinitions()}, nil)
	case "tools/call":
		s.handleToolCall(ctx, req)
	default:
		s.writeResponse(req.ID, nil, &jsonrpcError{Code: -32601, Message: "method not found: " + req.Method})
	}
}

func (s *Server) handleToolCall(ctx context.Context, req *jsonrpcRequest) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
		s.writeResponse(req.ID, nil, &jsonrpcError{Code: -32602, Message: "invalid tools/call parameters"})
		return
	}
	handler, ok := s.tools[params.Name]
	if !ok {
		s.writeResponse(req.ID, nil, &jsonrpcError{Code: -32602, Message: "unknown tool: " + params.Name})
		return
	}
	content, err := executeToolHandler(ctx, handler, params.Arguments)
	result := map[string]any{"content": []map[string]any{{"type": "text", "text": fmt.Sprint(content)}}}
	if err != nil {
		result["isError"] = true
		result["content"] = []map[string]any{{"type": "text", "text": err.Error()}}
	}
	s.writeResponse(req.ID, result, nil)
}

func executeToolHandler(ctx context.Context, handler ToolHandler, params map[string]any) (content any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("tool handler panic: %v", recovered)
		}
	}()
	return handler(ctx, params)
}

func (s *Server) writeResponse(id *json.RawMessage, result any, rpcErr *jsonrpcError) {
	data, err := json.Marshal(jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.writer.Write(append(data, '\n'))
}
