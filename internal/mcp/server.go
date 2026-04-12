// Package mcp implements a Model Context Protocol server over stdio.
// The server exposes kubecat's cluster context to MCP clients such as
// Claude Desktop via JSON-RPC 2.0.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Server handles JSON-RPC 2.0 requests over stdio.
type Server struct {
	handler *Handler
	in      io.Reader
	out     io.Writer
}

// NewServer creates a Server backed by the given handler.
// If in/out are nil, os.Stdin/os.Stdout are used.
func NewServer(h *Handler, in io.Reader, out io.Writer) *Server {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &Server{handler: h, in: in, out: out}
}

// Start reads JSON-RPC requests line-by-line until ctx is canceled or EOF.
func (s *Server) Start(ctx context.Context) {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max line

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !scanner.Scan() {
			return
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := s.dispatch(ctx, line)
		if resp == nil {
			continue
		}
		b, err := json.Marshal(resp)
		if err != nil {
			slog.Error("mcp: marshal response", slog.Any("error", err))
			continue
		}
		fmt.Fprintf(s.out, "%s\n", b)
	}
}

// rpcRequest is the inbound JSON-RPC 2.0 envelope.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is the outbound JSON-RPC 2.0 envelope.
type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) dispatch(ctx context.Context, raw []byte) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		}
	}

	base := &rpcResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		base.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo": map[string]interface{}{
				"name":    "kubecat",
				"version": "1.0.0",
			},
		}

	case "tools/list":
		base.Result = map[string]interface{}{"tools": ToolDefinitions()}

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = &rpcError{Code: -32602, Message: "invalid params"}
			return base
		}
		result, err := s.handler.Call(ctx, p.Name, p.Arguments)
		if err != nil {
			base.Error = &rpcError{Code: -32603, Message: err.Error()}
			return base
		}
		base.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result},
			},
		}

	default:
		if req.ID == nil {
			// Notification — no response required.
			return nil
		}
		base.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}

	return base
}
