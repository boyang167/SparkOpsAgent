package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type listGitRefsRequest struct {
	GitURL   string `json:"git_url"`
	SecretID string `json:"secret_id"`
	RefType  string `json:"ref_type"`
}

func (h *Handler) ListGitRefs(c *gin.Context) {
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
	var req listGitRefsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	items, err := h.listGitRefs.Handle(
		c.Request.Context(),
		wsID,
		memberID,
		strings.TrimSpace(req.GitURL),
		strings.TrimSpace(req.SecretID),
		strings.TrimSpace(req.RefType),
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type listGitTreeRequest struct {
	GitURL   string `json:"git_url"`
	SecretID string `json:"secret_id"`
	Ref      string `json:"ref"`
	Path     string `json:"path"`
}

func (h *Handler) ListGitTree(c *gin.Context) {
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
	var req listGitTreeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	items, err := h.listGitTree.Handle(
		c.Request.Context(),
		wsID,
		memberID,
		strings.TrimSpace(req.GitURL),
		strings.TrimSpace(req.SecretID),
		strings.TrimSpace(req.Ref),
		strings.TrimSpace(req.Path),
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type getGitFileContentRequest struct {
	GitURL   string `json:"git_url"`
	SecretID string `json:"secret_id"`
	Ref      string `json:"ref"`
	Path     string `json:"path"`
}

func (h *Handler) GetGitFileContent(c *gin.Context) {
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
	var req getGitFileContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	data, err := h.getGitFileContent.Handle(
		c.Request.Context(),
		wsID,
		memberID,
		strings.TrimSpace(req.GitURL),
		strings.TrimSpace(req.SecretID),
		strings.TrimSpace(req.Ref),
		strings.TrimSpace(req.Path),
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, data)
}
