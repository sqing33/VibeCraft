package mcpstdio

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type ToolHandler func(args json.RawMessage) (any, error)

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

type Server struct {
	name    string
	version string

	tools map[string]Tool
}

func NewServer(name, version string) *Server {
	return &Server{
		name:    strings.TrimSpace(name),
		version: strings.TrimSpace(version),
		tools:   make(map[string]Tool),
	}
}

func (s *Server) RegisterTool(tool Tool) {
	tool.Name = strings.TrimSpace(tool.Name)
	s.tools[tool.Name] = tool
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) ServeStdio(in io.Reader, out io.Writer) error {
	rd := NewReader(in)
	for {
		msg, framing, err := rd.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(msg) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			// Can't reply without id; ignore.
			continue
		}
		if strings.TrimSpace(req.Method) == "" {
			continue
		}

		// Notifications have no id.
		if len(req.ID) == 0 {
			continue
		}

		resp := s.handle(req)
		b, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		// Some clients (especially simple harnesses) only support line-delimited JSON.
		// Reply using the same framing style as the request.
		if framing == FramingLineDelimited {
			if err := WriteMessageLine(out, b); err != nil {
				return err
			}
			continue
		}
		if err := WriteMessage(out, b); err != nil {
			return err
		}
	}
}

func (s *Server) handle(req rpcRequest) rpcResponse {
	method := strings.TrimSpace(req.Method)
	switch method {
	case "initialize":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]any{
					"name":    s.name,
					"version": s.version,
				},
				"capabilities": map[string]any{
					"tools": map[string]any{
						"listChanged": false,
					},
				},
			},
		}
	case "tools/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: s.toolsListResult()}
	case "tools/call":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: s.toolsCallResult(req.Params)}
	default:
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}
	}
}

func (s *Server) toolsListResult() map[string]any {
	keys := make([]string, 0, len(s.tools))
	for k := range s.tools {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		tool := s.tools[k]
		item := map[string]any{
			"name": tool.Name,
		}
		if strings.TrimSpace(tool.Description) != "" {
			item["description"] = strings.TrimSpace(tool.Description)
		}
		if tool.InputSchema != nil {
			item["inputSchema"] = tool.InputSchema
		}
		out = append(out, item)
	}
	return map[string]any{"tools": out}
}

func (s *Server) toolsCallResult(rawParams json.RawMessage) map[string]any {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return toolErrorResult(fmt.Errorf("invalid params: %w", err))
	}
	name := strings.TrimSpace(params.Name)
	tool, ok := s.tools[name]
	if !ok {
		return toolErrorResult(fmt.Errorf("unknown tool: %s", name))
	}

	res, err := tool.Handler(params.Arguments)
	if err != nil {
		return toolErrorResult(err)
	}
	// MCP tool results are returned as content items. To keep "structured JSON",
	// we encode the tool output as JSON text so the model can parse it reliably.
	text, _ := json.MarshalIndent(res, "", "  ")
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(text) + "\n"},
		},
	}
}

func toolErrorResult(err error) map[string]any {
	msg := "error"
	if err != nil {
		msg = err.Error()
	}
	return map[string]any{
		"isError": true,
		"content": []map[string]any{
			{"type": "text", "text": msg + "\n"},
		},
	}
}
