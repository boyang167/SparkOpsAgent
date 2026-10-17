package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	identityhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type adminSSORepository struct {
	mu  sync.Mutex
	cfg model.SSOConfiguration
}

func (r *adminSSORepository) Load(context.Context) (*model.SSOConfiguration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyConfig := r.cfg
	return &copyConfig, nil
}

func (r *adminSSORepository) Save(
	_ context.Context,
	expectedVersion int64,
	cfg model.SSOConfiguration,
) (*model.SSOConfiguration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg.Version != expectedVersion {
		return nil, domain.ErrSSOVersionConflict
	}
	cfg.Version = expectedVersion + 1
	r.cfg = cfg
	copyConfig := cfg
	return &copyConfig, nil
}

func setupSSOAdminRouter(t *testing.T) (*gin.Engine, *adminSSORepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &adminSSORepository{cfg: model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Provider:          model.SSOProviderOIDC,
			ShowPasswordLogin: true,
		},
		OIDC: model.OIDCSettings{
			Issuer:       "https://idp.example.com",
			ClientID:     "sparkops",
			ClientSecret: "stored-client-secret",
			RedirectURL:  "https://sparkops.example.com/callback",
			Scopes:       "openid email profile",
		},
		LDAP: model.LDAPSettings{
			URL:              "ldaps://ldap.example.com",
			BindDN:           "cn=reader,dc=example,dc=com",
			BindPassword:     "stored-bind-password",
			UserSearchBase:   "ou=people,dc=example,dc=com",
			UserSearchFilter: "(uid=%s)",
			EmailAttribute:   "mail",
			NameAttribute:    "cn",
		},
		Version: 3,
	}}
	runtime := service.NewSSORuntime(repo, config.AuthConfig{}, http.DefaultClient)
	require.NoError(t, runtime.Load(context.Background()))
	manager := command.NewSSOConfigManager(repo, runtime)
	handler := identityhttp.NewSSOAdminHandler(manager)
	router := gin.New()
	router.GET("/api/v1/admin/sso", handler.Get)
	router.PUT("/api/v1/admin/sso", handler.Update)
	return router, repo
}

func TestSSOAdminGetMasksSecrets(t *testing.T) {
	router, _ := setupSSOAdminRouter(t)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/sso", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"has_client_secret":true`)
	require.Contains(t, recorder.Body.String(), `"has_bind_password":true`)
	require.NotContains(t, recorder.Body.String(), "stored-client-secret")
	require.NotContains(t, recorder.Body.String(), "stored-bind-password")
}

func TestSSOAdminUpdateKeepsStoredSecretsWhenInputsAreEmpty(t *testing.T) {
	router, repo := setupSSOAdminRouter(t)
	body := `{
		"version":3,
		"login_ui":{"enabled":false,"provider":"oidc","show_password_login":true},
		"oidc":{
			"enabled":false,"issuer":"https://idp.example.com","client_id":"sparkops",
			"client_secret":"","clear_client_secret":false,
			"redirect_url":"https://sparkops.example.com/callback","scopes":"openid email profile"
		},
		"ldap":{
			"enabled":false,"url":"ldaps://ldap.example.com",
			"bind_dn":"cn=reader,dc=example,dc=com","bind_password":"","clear_bind_password":false,
			"user_search_base":"ou=people,dc=example,dc=com","user_search_filter":"(uid=%s)",
			"email_attribute":"mail","name_attribute":"cn"
		}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sso", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "stored-client-secret", repo.cfg.OIDC.ClientSecret)
	require.Equal(t, "stored-bind-password", repo.cfg.LDAP.BindPassword)
	require.NotContains(t, recorder.Body.String(), "stored-client-secret")
	require.NotContains(t, recorder.Body.String(), "stored-bind-password")
}

func TestSSOAdminUpdateReturnsConflictForStaleVersion(t *testing.T) {
	router, _ := setupSSOAdminRouter(t)
	body := `{
		"version":2,
		"login_ui":{"enabled":false,"provider":"oidc","show_password_login":true},
		"oidc":{"enabled":false,"issuer":"","client_id":"","client_secret":"","redirect_url":"","scopes":""},
		"ldap":{"enabled":false,"url":"","bind_dn":"","bind_password":"","user_search_base":"","user_search_filter":"(uid=%s)","email_attribute":"mail","name_attribute":"cn"}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sso", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "IDENTITY_SSO_VERSION_CONFLICT")
}

func TestAuthProvidersReadsLatestRuntimeSnapshot(t *testing.T) {
	repo := &adminSSORepository{cfg: model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Provider:          model.SSOProviderLDAP,
			ShowPasswordLogin: true,
		},
		Version: 1,
	}}
	runtime := service.NewSSORuntime(repo, config.AuthConfig{}, http.DefaultClient)
	require.NoError(t, runtime.Load(context.Background()))
	handler := identityhttp.NewSSOHandler(runtime, nil, command.NewOIDCStateStore())
	router := gin.New()
	router.GET("/api/v1/auth/providers", handler.AuthProviders)

	enabled := repo.cfg
	enabled.LoginUI.Enabled = true
	enabled.LDAP = model.LDAPSettings{
		Enabled: true, URL: "ldaps://ldap.example.com",
		UserSearchBase: "ou=people,dc=example,dc=com", UserSearchFilter: "(uid=%s)",
		EmailAttribute: "mail", NameAttribute: "cn",
	}
	enabled.LDAPTestedFingerprint = enabled.ProviderFingerprint(model.SSOProviderLDAP)
	require.NoError(t, runtime.Replace(enabled))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"ldap_enabled":true`)
	require.Contains(t, recorder.Body.String(), `"provider":"ldap"`)
}
