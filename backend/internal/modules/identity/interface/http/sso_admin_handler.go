package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type SSOAdminHandler struct {
	manager *command.SSOConfigManager
}

func NewSSOAdminHandler(manager *command.SSOConfigManager) *SSOAdminHandler {
	return &SSOAdminHandler{manager: manager}
}

type loginUIDTO struct {
	Enabled           bool   `json:"enabled"`
	Provider          string `json:"provider"`
	ShowPasswordLogin bool   `json:"show_password_login"`
}

type oidcConfigDTO struct {
	Enabled               bool   `json:"enabled"`
	Issuer                string `json:"issuer"`
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"client_secret,omitempty"`
	ClearClientSecret     bool   `json:"clear_client_secret,omitempty"`
	RedirectURL           string `json:"redirect_url"`
	Scopes                string `json:"scopes"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	TestedFingerprint     string `json:"tested_fingerprint"`
	TestedAt              string `json:"tested_at,omitempty"`
	HasClientSecret       bool   `json:"has_client_secret"`
	RuntimeEnabled        bool   `json:"runtime_enabled"`
}

type ldapConfigDTO struct {
	Enabled           bool   `json:"enabled"`
	URL               string `json:"url"`
	BindDN            string `json:"bind_dn"`
	BindPassword      string `json:"bind_password,omitempty"`
	ClearBindPassword bool   `json:"clear_bind_password,omitempty"`
	UserSearchBase    string `json:"user_search_base"`
	UserSearchFilter  string `json:"user_search_filter"`
	EmailAttribute    string `json:"email_attribute"`
	NameAttribute     string `json:"name_attribute"`
	TestedFingerprint string `json:"tested_fingerprint"`
	TestedAt          string `json:"tested_at,omitempty"`
	HasBindPassword   bool   `json:"has_bind_password"`
	RuntimeEnabled    bool   `json:"runtime_enabled"`
}

type updateSSORequest struct {
	Version int64         `json:"version"`
	LoginUI loginUIDTO    `json:"login_ui" binding:"required"`
	OIDC    oidcConfigDTO `json:"oidc" binding:"required"`
	LDAP    ldapConfigDTO `json:"ldap" binding:"required"`
}

type testSSORequest struct {
	Provider string           `json:"provider" binding:"required"`
	Config   updateSSORequest `json:"config" binding:"required"`
}

type ssoAdminResponse struct {
	Version int64         `json:"version"`
	LoginUI loginUIDTO    `json:"login_ui"`
	OIDC    oidcConfigDTO `json:"oidc"`
	LDAP    ldapConfigDTO `json:"ldap"`
}

func (h *SSOAdminHandler) Get(c *gin.Context) {
	cfg, err := h.manager.Get(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, toSSOAdminResponse(*cfg))
}

func (h *SSOAdminHandler) Test(c *gin.Context) {
	var req testSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	cfg, secrets, err := req.Config.toDomain()
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.manager.Test(
		c.Request.Context(),
		model.SSOProvider(strings.ToLower(strings.TrimSpace(req.Provider))),
		cfg,
		secrets,
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"provider": result.Provider, "fingerprint": result.Fingerprint,
		"tested_at": result.TestedAt.Format(time.RFC3339), "message": result.Message,
	})
}

func (h *SSOAdminHandler) Update(c *gin.Context) {
	var req updateSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	cfg, secrets, err := req.toDomain()
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	saved, err := h.manager.Save(c.Request.Context(), req.Version, cfg, secrets)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, toSSOAdminResponse(*saved))
}

func (r updateSSORequest) toDomain() (model.SSOConfiguration, command.SSOSecretUpdate, error) {
	cfg := model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Enabled: r.LoginUI.Enabled, Provider: model.SSOProvider(r.LoginUI.Provider),
			ShowPasswordLogin: r.LoginUI.ShowPasswordLogin,
		},
		OIDC: model.OIDCSettings{
			Enabled: r.OIDC.Enabled, Issuer: r.OIDC.Issuer, ClientID: r.OIDC.ClientID,
			ClientSecret: r.OIDC.ClientSecret, RedirectURL: r.OIDC.RedirectURL, Scopes: r.OIDC.Scopes,
			AuthorizationEndpoint: r.OIDC.AuthorizationEndpoint, TokenEndpoint: r.OIDC.TokenEndpoint,
			UserInfoEndpoint: r.OIDC.UserInfoEndpoint,
		},
		LDAP: model.LDAPSettings{
			Enabled: r.LDAP.Enabled, URL: r.LDAP.URL, BindDN: r.LDAP.BindDN,
			BindPassword: r.LDAP.BindPassword, UserSearchBase: r.LDAP.UserSearchBase,
			UserSearchFilter: r.LDAP.UserSearchFilter, EmailAttribute: r.LDAP.EmailAttribute,
			NameAttribute: r.LDAP.NameAttribute,
		},
		OIDCTestedFingerprint: r.OIDC.TestedFingerprint,
		LDAPTestedFingerprint: r.LDAP.TestedFingerprint,
		Version:               r.Version,
	}
	var err error
	cfg.OIDCTestedAt, err = parseOptionalTime(r.OIDC.TestedAt)
	if err != nil {
		return model.SSOConfiguration{}, command.SSOSecretUpdate{}, err
	}
	cfg.LDAPTestedAt, err = parseOptionalTime(r.LDAP.TestedAt)
	if err != nil {
		return model.SSOConfiguration{}, command.SSOSecretUpdate{}, err
	}
	return cfg, command.SSOSecretUpdate{
		ClearClientSecret: r.OIDC.ClearClientSecret,
		ClearBindPassword: r.LDAP.ClearBindPassword,
	}, nil
}

func toSSOAdminResponse(cfg model.SSOConfiguration) ssoAdminResponse {
	return ssoAdminResponse{
		Version: cfg.Version,
		LoginUI: loginUIDTO{
			Enabled: cfg.LoginUI.Enabled, Provider: string(cfg.LoginUI.Provider),
			ShowPasswordLogin: cfg.LoginUI.ShowPasswordLogin,
		},
		OIDC: oidcConfigDTO{
			Enabled: cfg.OIDC.Enabled, Issuer: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID,
			RedirectURL: cfg.OIDC.RedirectURL, Scopes: cfg.OIDC.Scopes,
			AuthorizationEndpoint: cfg.OIDC.AuthorizationEndpoint, TokenEndpoint: cfg.OIDC.TokenEndpoint,
			UserInfoEndpoint: cfg.OIDC.UserInfoEndpoint, TestedFingerprint: cfg.OIDCTestedFingerprint,
			TestedAt: formatOptionalTime(cfg.OIDCTestedAt), HasClientSecret: cfg.OIDC.ClientSecret != "",
			RuntimeEnabled: cfg.OIDC.Enabled,
		},
		LDAP: ldapConfigDTO{
			Enabled: cfg.LDAP.Enabled, URL: cfg.LDAP.URL, BindDN: cfg.LDAP.BindDN,
			UserSearchBase: cfg.LDAP.UserSearchBase, UserSearchFilter: cfg.LDAP.UserSearchFilter,
			EmailAttribute: cfg.LDAP.EmailAttribute, NameAttribute: cfg.LDAP.NameAttribute,
			TestedFingerprint: cfg.LDAPTestedFingerprint, TestedAt: formatOptionalTime(cfg.LDAPTestedAt),
			HasBindPassword: cfg.LDAP.BindPassword != "", RuntimeEnabled: cfg.LDAP.Enabled,
		},
	}
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
