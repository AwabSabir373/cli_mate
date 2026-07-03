// Package acp implements the Agent Client Protocol, a JSON-RPC server
// that exposes cli_mate's capabilities to editors like VS Code.
package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"sync"
)

// Server is a JSON-RPC server for editor integration.
type Server struct {
	listener net.Listener
	server   *rpc.Server
	handler  Handler
	mu       sync.Mutex
	running  bool
}

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  interface{}   `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RunRequest is the params for the "run" method.
type RunRequest struct {
	Prompt string `json:"prompt"`
	Cwd    string `json:"cwd,omitempty"`
	Model  string `json:"model,omitempty"`
}

// RunResult is the result of the "run" method.
type RunResult struct {
	Answer string `json:"answer"`
	Steps  int    `json:"steps"`
}

// StatusResult is the result of the "status" method.
type StatusResult struct {
	Version   string `json:"version"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Workspace string `json:"workspace"`
}

// ToolsResult is the result of the "list_tools" method.
type ToolsResult struct {
	Tools []ToolInfo `json:"tools"`
}

// ToolInfo describes a single tool.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Handler processes ACP requests.
type Handler interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	Status(ctx context.Context) (*StatusResult, error)
	ListTools(ctx context.Context) (*ToolsResult, error)
	Cancel(ctx context.Context) error
}

// NewServer creates a new ACP server on the given socket path.
func NewServer(socketPath string, handler Handler) (*Server, error) {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("acp: listen: %w", err)
	}

	server := rpc.NewServer()
	svc := &service{handler: handler}
	server.Register(svc)

	return &Server{
		listener: listener,
		server:   server,
		handler:  handler,
	}, nil
}

// Serve starts accepting connections. Blocks until the server is closed.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return nil
			}
			continue
		}
		go s.handleConn(conn)
	}
}

// Close stops the server.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
}

// SocketPath returns the path the server is listening on.
func (s *Server) SocketPath() string {
	return s.listener.Addr().String()
}

func (s *Server) handleConn(conn io.ReadWriteCloser) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			return
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		result, err := s.handleMethod(context.Background(), req.Method, req.Params)
		if err != nil {
			resp.Error = &Error{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}

		encoder.Encode(resp)
	}
}

func (s *Server) handleMethod(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if s.handler == nil {
		return nil, fmt.Errorf("handler not configured")
	}

	switch method {
	case "run":
		var req RunRequest
		if params != nil {
			data, _ := json.Marshal(params)
			json.Unmarshal(data, &req)
		}
		return s.handler.Run(ctx, req)
	case "status":
		return s.handler.Status(ctx)
	case "list_tools":
		return s.handler.ListTools(ctx)
	case "cancel":
		return nil, s.handler.Cancel(ctx)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

// service is the RPC service implementation.
type service struct {
	handler Handler
}

func (s *service) Run(ctx context.Context, req RunRequest, reply *RunResult) error {
	if s.handler == nil {
		return fmt.Errorf("handler not configured")
	}
	result, err := s.handler.Run(ctx, req)
	if err != nil {
		return err
	}
	*reply = *result
	return nil
}

func (s *service) Status(ctx context.Context, reply *StatusResult) error {
	if s.handler == nil {
		return fmt.Errorf("handler not configured")
	}
	result, err := s.handler.Status(ctx)
	if err != nil {
		return err
	}
	*reply = *result
	return nil
}

func (s *service) ListTools(ctx context.Context, reply *ToolsResult) error {
	if s.handler == nil {
		return fmt.Errorf("handler not configured")
	}
	result, err := s.handler.ListTools(ctx)
	if err != nil {
		return err
	}
	*reply = *result
	return nil
}

func (s *service) Cancel(ctx context.Context, reply *struct{}) error {
	if s.handler == nil {
		return fmt.Errorf("handler not configured")
	}
	return s.handler.Cancel(ctx)
}
