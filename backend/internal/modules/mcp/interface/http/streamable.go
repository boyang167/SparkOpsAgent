package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

// HandleStreamable implements MCP Streamable HTTP (2025-03-26) on a single endpoint.
// GET returns 405 when the server does not offer a standalone SSE listener.
func (h *Handler) HandleStreamable(c *gin.Context) {
	if !validateMCPOrigin(c.GetHeader("Origin")) {
		httpx.Fail(c, "MCP_ORIGIN_REJECTED", "origin not allowed", http.StatusForbidden)
		return
	}

	switch c.Request.Method {
	case http.MethodGet:
		c.Header("Allow", "POST, DELETE")
		httpx.Fail(c, "MCP_METHOD_NOT_ALLOWED", "SSE stream not supported on this endpoint", http.StatusMethodNotAllowed)
		return
	case http.MethodDelete:
		c.Header("Allow", "POST, DELETE")
		c.Status(http.StatusMethodNotAllowed)
		return
	case http.MethodPost:
		h.handleStreamablePost(c)
	default:
		c.Header("Allow", "POST, DELETE")
		c.Status(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleStreamablePost(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if len(raw) == 0 {
		httpx.Fail(c, "INVALID_REQUEST", "empty body", http.StatusBadRequest)
		return
	}

	messages, err := decodeStreamableMessages(raw)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}

	var requests []application.RPCRequest
	var onlyResponsesOrNotifications = true
	for _, msg := range messages {
		switch msg.kind {
		case messageRequest:
			onlyResponsesOrNotifications = false
			requests = append(requests, msg.request)
		case messageNotification:
			// accepted below
		case messageResponse:
			// client → server responses (e.g. server-initiated RPC) — accept with 202
		default:
			httpx.Fail(c, "INVALID_REQUEST", "invalid json-rpc message", http.StatusBadRequest)
			return
		}
	}

	if onlyResponsesOrNotifications {
		c.AbortWithStatus(http.StatusAccepted)
		return
	}

	responses := make([]application.RPCResponse, 0, len(requests))
	for _, req := range requests {
		if req.JSONRPC == "" {
			req.JSONRPC = "2.0"
		}
		responses = append(responses, h.rpc.Handle(c.Request.Context(), wsID, memberID, req))
	}

	c.Header("Content-Type", "application/json")
	if len(responses) == 1 {
		c.JSON(http.StatusOK, responses[0])
		return
	}
	c.JSON(http.StatusOK, responses)
}

type messageKind int

const (
	messageUnknown messageKind = iota
	messageRequest
	messageNotification
	messageResponse
)

type decodedMessage struct {
	kind    messageKind
	request application.RPCRequest
}

func decodeStreamableMessages(raw []byte) ([]decodedMessage, error) {
	if raw[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		out := make([]decodedMessage, 0, len(batch))
		for _, item := range batch {
			msg, err := classifyStreamableMessage(item)
			if err != nil {
				return nil, err
			}
			out = append(out, msg)
		}
		return out, nil
	}
	msg, err := classifyStreamableMessage(raw)
	if err != nil {
		return nil, err
	}
	return []decodedMessage{msg}, nil
}

func classifyStreamableMessage(raw json.RawMessage) (decodedMessage, error) {
	var probe struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return decodedMessage{}, err
	}

	hasID := len(probe.ID) > 0 && string(probe.ID) != "null"
	hasMethod := probe.Method != ""
	hasResultOrErr := len(probe.Result) > 0 || len(probe.Error) > 0

	switch {
	case hasMethod && hasID:
		var req application.RPCRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return decodedMessage{}, err
		}
		return decodedMessage{kind: messageRequest, request: req}, nil
	case hasMethod && !hasID:
		return decodedMessage{kind: messageNotification}, nil
	case !hasMethod && hasID && hasResultOrErr:
		return decodedMessage{kind: messageResponse}, nil
	default:
		return decodedMessage{kind: messageUnknown}, nil
	}
}

func validateMCPOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" {
		return true
	}
	lower := strings.ToLower(origin)
	if strings.HasPrefix(lower, "http://localhost:") ||
		strings.HasPrefix(lower, "http://127.0.0.1:") ||
		strings.HasPrefix(lower, "https://localhost:") ||
		strings.HasPrefix(lower, "https://127.0.0.1:") {
		return true
	}
	if lower == "http://localhost" || lower == "http://127.0.0.1" {
		return true
	}
	return false
}
