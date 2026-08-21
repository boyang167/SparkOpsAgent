package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	idauth "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/auth"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type SSOHandler struct {
	db       *gorm.DB
	oidc     *idauth.OIDCClient
	ldap     *command.LDAPLoginHandler
	resolver *command.ResolveExternalIdentityHandler
	states   *command.OIDCStateStore
}

func NewSSOHandler(
	db *gorm.DB,
	oidc *idauth.OIDCClient,
	ldap *command.LDAPLoginHandler,
	resolver *command.ResolveExternalIdentityHandler,
	states *command.OIDCStateStore,
) *SSOHandler {
	return &SSOHandler{db: db, oidc: oidc, ldap: ldap, resolver: resolver, states: states}
}

func (h *SSOHandler) AuthProviders(c *gin.Context) {
	ui, _ := entitlements.LoadLoginUIConfig(c.Request.Context(), h.db)
	httpx.OK(c, gin.H{
		"oidc_enabled":        h.oidc != nil && h.oidc.Enabled(),
		"ldap_enabled":        h.ldap != nil,
		"sso_ui":              ui,
	})
}

func (h *SSOHandler) OIDCLogin(c *gin.Context) {
	if h.oidc == nil || !h.oidc.Enabled() {
		httpx.Fail(c, "AUTH_OIDC_DISABLED", "oidc login disabled", http.StatusBadRequest)
		return
	}
	state, err := idauth.NewOAuthState()
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	h.states.Put(state)
	httpx.OK(c, gin.H{"authorization_url": h.oidc.AuthURL(state), "state": state})
}

func (h *SSOHandler) OIDCCallback(c *gin.Context) {
	if h.oidc == nil || !h.oidc.Enabled() {
		httpx.Fail(c, "AUTH_OIDC_DISABLED", "oidc login disabled", http.StatusBadRequest)
		return
	}
	state := c.Query("state")
	code := c.Query("code")
	if !h.states.Consume(state) {
		httpx.Fail(c, "AUTH_OIDC_INVALID_STATE", "invalid oauth state", http.StatusBadRequest)
		return
	}
	user, err := h.oidc.ExchangeUser(c.Request.Context(), code)
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
	if h.ldap == nil {
		httpx.Fail(c, "AUTH_LDAP_DISABLED", "ldap login disabled", http.StatusBadRequest)
		return
	}
	var req ldapLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.ldap.Handle(c.Request.Context(), req.Username, req.Password)
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
