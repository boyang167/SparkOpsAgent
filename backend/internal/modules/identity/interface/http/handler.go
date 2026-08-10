package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Handler struct {
	register       *command.RegisterMemberHandler
	login          *command.LoginHandler
	me             *query.GetCurrentMemberHandler
	updateProfile  *command.UpdateProfileHandler
	changePassword *command.ChangePasswordHandler
}

func NewHandler(
	register *command.RegisterMemberHandler,
	login *command.LoginHandler,
	me *query.GetCurrentMemberHandler,
	updateProfile *command.UpdateProfileHandler,
	changePassword *command.ChangePasswordHandler,
) *Handler {
	return &Handler{
		register:       register,
		login:          login,
		me:             me,
		updateProfile:  updateProfile,
		changePassword: changePassword,
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.register.Handle(c.Request.Context(), command.RegisterMemberInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, RegisterResponse{ID: result.MemberID.String(), Email: result.Email})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.login.Handle(c.Request.Context(), command.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, LoginResponse{
		Token: result.Token,
		ID:    result.MemberID.String(),
		Email: result.Email,
	})
}

func (h *Handler) Me(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	view, err := h.me.Handle(c.Request.Context(), memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	member, err := h.updateProfile.Handle(c.Request.Context(), memberID, req.DisplayName)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": member.ID.String(), "email": member.Email, "display_name": member.DisplayName,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.changePassword.Handle(c.Request.Context(), memberID, req.CurrentPassword, req.NewPassword); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"changed": true})
}
