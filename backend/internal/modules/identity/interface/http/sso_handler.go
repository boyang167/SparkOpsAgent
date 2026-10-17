package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/service"
	idauth "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/auth"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type SSOHandler struct {
	runtime  *service.SSORuntime
	resolver *command.ResolveExternalIdentityHandler
	states   *command.OIDCStateStore
}

func NewSSOHandler(
	runtime *service.SSORuntime,
	resolver *command.ResolveExternalIdentityHandler,
	states *command.OIDCStateStore,
) *SSOHandler {
	return &SSOHandler{runtime: runtime, resolver: resolver, states: states}
}

func (h *SSOHandler) AuthProviders(c *gin.Context) {
	snapshot := h.runtime.Snapshot()
	ui := snapshot.Configuration.LoginUI
	httpx.OK(c, gin.H{
		"oidc_enabled": snapshot.OIDC != nil && snapshot.OIDC.Enabled(),
		"ldap_enabled": snapshot.LDAP != nil,
		"sso_ui": gin.H{
			"enabled": ui.Enabled, "provider": ui.Provider,
			"show_password_login": ui.ShowPasswordLogin,
		},
	})
}

func (h *SSOHandler) OIDCLogin(c *gin.Context) {
	snapshot := h.runtime.Snapshot()
	if snapshot.OIDC == nil || !snapshot.OIDC.Enabled() {
		httpx.Fail(c, "AUTH_OIDC_DISABLED", "oidc login disabled", http.StatusBadRequest)
		return
	}
	state, err := idauth.NewOAuthState()
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	authorizationURL, err := snapshot.OIDC.AuthURLContext(c.Request.Context(), state)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	h.states.Put(state)
	httpx.OK(c, gin.H{"authorization_url": authorizationURL, "state": state})
}

func (h *SSOHandler) OIDCCallback(c *gin.Context) {
	snapshot := h.runtime.Snapshot()
	if snapshot.OIDC == nil || !snapshot.OIDC.Enabled() {
		httpx.Fail(c, "AUTH_OIDC_DISABLED", "oidc login disabled", http.StatusBadRequest)
		return
	}
	state := c.Query("state")
	code := c.Query("code")
	if !h.states.Consume(state) {
		httpx.Fail(c, "AUTH_OIDC_INVALID_STATE", "invalid oauth state", http.StatusBadRequest)
		return
	}
	user, err := snapshot.OIDC.ExchangeUser(c.Request.Context(), code)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	result, err := h.resolver.Handle(c.Request.Context(), idauth.OIDCProvider, user.Subject, user.Email, user.DisplayName)
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

type ldapLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *SSOHandler) LDAPLogin(c *gin.Context) {
	snapshot := h.runtime.Snapshot()
	if snapshot.LDAP == nil {
		httpx.Fail(c, "AUTH_LDAP_DISABLED", "ldap login disabled", http.StatusBadRequest)
		return
	}
	var req ldapLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	user, err := snapshot.LDAP.AuthenticateContext(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	result, err := h.resolver.Handle(
		c.Request.Context(),
		"ldap",
		user.Subject,
		user.Email,
		user.DisplayName,
	)
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
