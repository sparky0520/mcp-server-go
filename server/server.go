package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sparky0520/mcp-server-go/jsonrpc"
)

const ProtocolVersion = "2025-06-18"

// ToolDefinition defines a tool that can be called by a client
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolHandler is a function that implements the logic of a tool
type ToolHandler func(ctx context.Context, arguments json.RawMessage) (any, error)

// Server implements the MCP protocol over JSON-RPC 2.0
type Server struct {
	in          io.Reader
	out         io.Writer
	mu          sync.RWMutex
	tools       map[string]ToolHandler
	definitions map[string]ToolDefinition
	initialized bool
	handlers    map[string]func(context.Context, jsonrpc.Request) error
	logger      *slog.Logger
}

func New(in io.Reader, out io.Writer, logger *slog.Logger) *Server {
	s := &Server{
		in:          in,
		out:         out,
		tools:       make(map[string]ToolHandler),
		definitions: make(map[string]ToolDefinition),
		logger:      logger,
	}

	s.handlers = map[string]func(context.Context, jsonrpc.Request) error{
		"initialize":                s.handleInitialize,
		"notifications/initialized": s.handleInitializedNotification,
		"ping":                      s.handlePing,
		"tools/list":                s.handleToolsList,
		"tools/call":                s.handleToolsCall,
	}

	return s
}

func (s *Server) RegisterTool(def ToolDefinition, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.definitions[def.Name] = def
	s.tools[def.Name] = handler
}

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP Server started", slog.String("protocolVersion", ProtocolVersion))
	defer s.logger.Info("MCP Server stopped")

	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// scanCh delivers lines from stdin so we can select on ctx.Done()
	scanCh := make(chan []byte)
	scanErr := make(chan error, 1)
	go func() {
		defer close(scanCh)
		for scanner.Scan() {
			line := make([]byte, len(scanner.Bytes()))
			copy(line, scanner.Bytes())
			scanCh <- line
		}
		scanErr <- scanner.Err()
	}()

	for {
		var line []byte
		select {
		case <-ctx.Done():
			return ctx.Err()
		case l, ok := <-scanCh:
			if !ok {
				if err := <-scanErr; err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				return nil
			}
			line = l
		}

		if len(line) == 0 {
			continue
		}

		// parse JSON, validate version, dispatch to handler
		var req jsonrpc.Request

		// 🔍 RIGHT HERE: Protocol Version Validation
		if req.JSONRPC != "2.0" {
			s.writeResponse(jsonrpc.Response{
				JSONRPC: "2.0",
				ID:      req.ID, // Fallback to whatever ID they provided (or nil)
				Error: &jsonrpc.Error{
					Code:    jsonrpc.InvalidRequest,
					Message: "Invalid Request: missing or incorrect jsonrpc version",
				},
			})
			continue // Skip processing this bad request
		}

		if err := json.Unmarshal(line, &req); err != nil {
			s.logger.Error("failed to unmarshal request", slog.String("error", err.Error()))
			// Write a standard JSON-RPC ParseError error back to the client
			s.writeResponse(jsonrpc.Response{
				JSONRPC: "2.0",
				Error: &jsonrpc.Error{
					Code:    jsonrpc.ParseError,
					Message: "Parse error",
				},
			})
			continue // skip to next loop
		}

		if err := s.handleRequest(ctx, req); err != nil {
			if errors.Is(err, errNotInitialized) {
				continue
			}
			return fmt.Errorf("failed to handle request: %w", err)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req jsonrpc.Request) error {
	handler, ok := s.handlers[req.Method]
	if ok {
		return handler(ctx, req)
	}

	// Unknown notification (no ID) -- silent ignore as per spec
	if req.ID == nil {
		s.logger.Debug("ignoring unknown notification", slog.String("method", req.Method))
		return nil
	}

	// Unknown method with ID -- respond with MethodNotFound
	return s.writeResponse(jsonrpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &jsonrpc.Error{
			Code:    jsonrpc.MethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		},
	})
}

func (s *Server) handleInitialize(ctx context.Context, req jsonrpc.Request) error {
	s.logger.Info("initialize request received", slog.Any("id", req.ID))

	type initializeParams struct {
		ProtocolVersion string         `json:"protocolVersion"`
		Capabilities    map[string]any `json:"capabilities"`
		ClientInfo      map[string]any `json:"clientInfo"`
	}

	var params initializeParams

	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeResponse(jsonrpc.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &jsonrpc.Error{
					Code:    jsonrpc.InvalidParams,
					Message: fmt.Sprintf("invalid initialize params: %v", err),
				},
			})
		}
	}

	return s.writeResponse(jsonrpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]any{
				"name":    "mcp-server-go",
				"version": "0.1.0",
			},
			"instructions": "This educational MCP server provides hello_world, health_check and latency_percentiles tools.",
		},
	})
}

func (s *Server) handleInitializedNotification(ctx context.Context, req jsonrpc.Request) error {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	s.logger.Info("initialized notification received")
	return nil
}

var errNotInitialized = errors.New("server not initialized")

func (s *Server) requireInitialized(req jsonrpc.Request) error {
	s.mu.Lock()
	initialized := s.initialized
	s.mu.Unlock()

	if initialized {
		return nil
	}

	if err := s.writeResponse(jsonrpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &jsonrpc.Error{
			Code:    jsonrpc.InvalidRequest,
			Message: "server has not received notifications/initialized yet",
		},
	}); err != nil {
		return err
	}

	return errNotInitialized
}

func (s *Server) handleToolsList(ctx context.Context, req jsonrpc.Request) error {
	if err := s.requireInitialized(req); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	defs := make([]ToolDefinition, 0, len(s.definitions))
	for _, def := range s.definitions {
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	return s.writeResponse(jsonrpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": defs,
		},
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req jsonrpc.Request) error {
	if err := s.requireInitialized(req); err != nil {
		return err
	}

	type callParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.writeResponse(jsonrpc.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonrpc.Error{
				Code:    jsonrpc.InvalidParams,
				Message: fmt.Sprintf("invalid tools/call params: %v", err),
			},
		})
	}

	s.mu.RLock()
	handler, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return s.writeResponse(jsonrpc.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonrpc.Error{
				Code:    jsonrpc.InvalidParams,
				Message: fmt.Sprintf("unknown tool: %s", params.Name),
			},
		})
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := handler(callCtx, params.Arguments)
	if err != nil {
		return s.writeResponse(jsonrpc.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": err.Error()},
				},
				"isError": true,
			},
		})
	}

	pretty, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to indent json response: %w", err)
	}

	return s.writeResponse(jsonrpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(pretty)},
			},
			"structuredContent": result,
			"isError":           false,
		},
	})
}

func (s *Server) writeResponse(resp jsonrpc.Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert the go struct into a single line json
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshalling response: %w", err)
	}

	// Append a newline character because MCP transport requires one message per line
	data = append(data, '\n')

	// Write it out to the client (usually os.Stdout)
	_, err = s.out.Write(data)
	return err
}
