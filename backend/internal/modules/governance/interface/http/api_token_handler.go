package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	identitycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type APITokenHandler struct {
	create *identitycommand.CreateAPITokenHandler
	revoke *identitycommand.RevokeAPITokenHandler
	store  *identitypersistence.APITokenStore
}

func NewAPITokenHandler(
	create *identitycommand.CreateAPITokenHandler,
	revoke *identitycommand.RevokeAPITokenHandler,
	store *identitypersistence.APITokenStore,
) *APITokenHandler {
	return &APITokenHandler{create: create, revoke: revoke, store: store}
}

func (h *APITokenHandler) List(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.store.ListByWorkspace(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *APITokenHandler) Create(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req struct {
		MemberID   string  `json:"member_id" binding:"required"`
		RoleID     *string `json:"role_id"`
		Remark     string  `json:"remark"`
		ExpireDays int     `json:"expire_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	memberID, err := kernel.ParseMemberID(req.MemberID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member_id", http.StatusBadRequest)
		return
	}
	var roleID *kernel.RoleID
	if req.RoleID != nil && *req.RoleID != "" {
		rid, err := kernel.ParseRoleID(*req.RoleID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid role_id", http.StatusBadRequest)
			return
		}
		roleID = &rid
	}
	result, err := h.create.Handle(c.Request.Context(), wsID, memberID, roleID, req.Remark, req.ExpireDays)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": result.ID, "token": result.Token, "token_prefix": result.TokenPrefix,
		"expires_at": result.ExpiresAt,
	})
}

func (h *APITokenHandler) Revoke(c *gin.Context) {
	tokenID := c.Param("tokenId")
	if err := h.revoke.Handle(c.Request.Context(), tokenID, ""); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"revoked": true})
}
