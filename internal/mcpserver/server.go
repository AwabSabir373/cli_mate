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

// Server handles the MCP JSON-RPC 2.0 communication over stdio.
type Server struct {
	reader  *bufio.Reader
	writer  io.Writer
	mu      sync.Mutex
	tools   map[string]ToolHandler

	// cancellation map
	activeReqs   map[int]context.CancelFunc
	activeReqsMu sync.Mutex

	// worker pool
	workQueue chan *jsonrpcRequest
}

// ToolHandler represents a function that handles a specific MCP tool call.
type ToolHandler func(ctx context.Context, params map[string]any) (any, error)

// NewServer creates a new MCP server with a bounded worker pool.
func NewServer() *Server {
	s := &Server{
		reader:     bufio.NewReader(os.Stdin),
		writer:     os.Stdout,
		tools:      make(map[string]ToolHandler),
		activeReqs: make(map[int]context.CancelFunc),
		workQueue:  make(chan *jsonrpcRequest, 100),
	}

	// Start bounded worker pool (10 concurrent requests)
	for i := 0; i < 10; i++ {
		go s.worker()
	}

	return s
}

// RegisterTool registers a new tool handler.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// Start begins listening for requests on stdin.
func (s *Server) Start() error {
	for {
		req, err := s.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Handle notifications immediately on the read goroutine
		if req.ID == nil {
			if req.Method == "notifications/cancelled" {
				var cancelParams struct {
					RequestID int `json:"requestId"`
				}
				if json.Unmarshal(req.Params, &cancelParams) == nil {
					s.cancelRequest(cancelParams.RequestID)
				}
			}
			continue
		}

		// Push requests to the bounded worker pool
		s.workQueue <- req
	}
}

func (s *Server) worker() {
	for req := range s.workQueue {
		s.handleRequest(req)
	}
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) readRequest() (*jsonrpcRequest, error) {
	contentLength := -1
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var req jsonrpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return &req, nil
}

func (s *Server) cancelRequest(id int) {
	s.activeReqsMu.Lock()
	defer s.activeReqsMu.Unlock()
	if cancel, exists := s.activeReqs[id]; exists {
		cancel()
		delete(s.activeReqs, id)
	}
}

func (s *Server) handleRequest(req *jsonrpcRequest) {
	if req.ID == nil {
		return
	}
	
	reqID := *req.ID
	ctx, cancel := context.WithCancel(context.Background())
	
	s.activeReqsMu.Lock()
	s.activeReqs[reqID] = cancel
	s.activeReqsMu.Unlock()

	defer func() {
		s.cancelRequest(reqID)
	}()

	var result any
	var err error

	switch req.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"serverInfo": map[string]any{
				"name":    "cli_mcp",
				"version": "1.1.0",
			},
		}
	case "tools/list":
		result = map[string]any{
			"tools": GetToolDefinitions(),
		}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if e := json.Unmarshal(req.Params, &params); e != nil {
			err = fmt.Errorf("invalid params: %w", e)
		} else {
			handler, ok := s.tools[params.Name]
			if !ok {
				err = fmt.Errorf("unknown tool: %s", params.Name)
			} else {
				content, e := handler(ctx, params.Arguments)
				if e != nil {
					result = map[string]any{
						"isError": true,
						"content": []map[string]any{
							{
								"type": "text",
								"text": e.Error(),
							},
						},
					}
				} else {
					result = map[string]any{
						"content": []map[string]any{
							{
								"type": "text",
								"text": fmt.Sprintf("%v", content),
							},
						},
					}
				}
			}
		}
	default:
		err = fmt.Errorf("method not found")
	}

	if result != nil && err == nil {
		s.writeResponse(req.ID, result, nil)
	} else if err != nil {
		s.writeResponse(req.ID, nil, &jsonrpcError{Code: -32601, Message: err.Error()})
	}
}

func (s *Server) writeResponse(id *int, result any, rpcErr *jsonrpcError) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.writer.Write([]byte(header))
	_, _ = s.writer.Write(data)
}
