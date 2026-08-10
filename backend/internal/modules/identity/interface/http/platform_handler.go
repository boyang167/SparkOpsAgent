package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PlatformHandler struct {
	list   *query.ListPlatformMembersHandler
	create *command.CreatePlatformMemberHandler
	update *command.UpdatePlatformMemberHandler
	reset  *command.AdminResetPasswordHandler
}

func NewPlatformHandler(
	list *query.ListPlatformMembersHandler,
	create *command.CreatePlatformMemberHandler,
	update *command.UpdatePlatformMemberHandler,
	reset *command.AdminResetPasswordHandler,
) *PlatformHandler {
	return &PlatformHandler{list: list, create: create, update: update, reset: reset}
}

func (h *PlatformHandler) ListMembers(c *gin.Context) {
	items, err := h.list.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type createPlatformMemberRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	DisplayName  string `json:"display_name" binding:"required"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

func (h *PlatformHandler) CreateMember(c *gin.Context) {
	var req createPlatformMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	res, err := h.create.Handle(c.Request.Context(), command.CreatePlatformMemberInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
		IsSuperAdmin: req.IsSuperAdmin,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": res.MemberID.String(), "email": res.Email, "is_super_admin": req.IsSuperAdmin,
	})
}

type updatePlatformMemberRequest struct {
	DisplayName  *string `json:"display_name"`
	Status       *string `json:"status"`
	IsSuperAdmin *bool   `json:"is_super_admin"`
}

func (h *PlatformHandler) UpdateMember(c *gin.Context) {
	actorID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	var req updatePlatformMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.update.Handle(c.Request.Context(), actorID, targetID, command.UpdatePlatformMemberInput{
		DisplayName: req.DisplayName, Status: req.Status, IsSuperAdmin: req.IsSuperAdmin,
	}); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

type adminResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *PlatformHandler) ResetPassword(c *gin.Context) {
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	var req adminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.reset.Handle(c.Request.Context(), targetID, req.NewPassword); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}
