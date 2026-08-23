package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RPCHandler struct {
	tools *ToolExecutor
}

func NewRPCHandler(tools *ToolExecutor) *RPCHandler {
	return &RPCHandler{tools: tools}
}

func (h *RPCHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	req RPCRequest,
) RPCResponse {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return h.err(req.ID, -32600, "invalid jsonrpc version")
	}

	switch req.Method {
	case "initialize":
		return h.ok(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "sparkops",
				"version": "0.1.0",
			},
		})
	case "notifications/initialized", "initialized":
		return h.ok(req.ID, map[string]any{})
	case "tools/list":
		defs := domain.AllTools()
		tools := make([]map[string]any, 0, len(defs))
		for _, tool := range defs {
			tools = append(tools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
		return h.ok(req.ID, map[string]any{"tools": tools})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return h.err(req.ID, -32602, "invalid params")
		}
		if params.Name == "" {
			return h.err(req.ID, -32602, "tool name required")
		}
		payload, err := h.tools.Call(ctx, wsID, memberID, params.Name, params.Arguments)
		if err != nil {
			return h.toolErr(req.ID, err)
		}
		text, _ := json.MarshalIndent(payload, "", "  ")
		return h.ok(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(text)},
			},
			"isError": false,
		})
	case "ping":
		return h.ok(req.ID, map[string]any{})
	default:
		return h.err(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (h *RPCHandler) ok(id any, result any) RPCResponse {
	return RPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func (h *RPCHandler) err(id any, code int, message string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
}

func (h *RPCHandler) toolErr(id any, err error) RPCResponse {
	text := err.Error()
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
			"isError": true,
		},
	}
}
