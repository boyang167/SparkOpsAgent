package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type memorySSOConfigRepository struct {
	mu  sync.Mutex
	cfg *model.SSOConfiguration
}

func (r *memorySSOConfigRepository) Load(context.Context) (*model.SSOConfiguration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg == nil {
		return nil, nil
	}
	copyConfig := *r.cfg
	return &copyConfig, nil
}

func (r *memorySSOConfigRepository) Save(
	_ context.Context,
	_ int64,
	cfg model.SSOConfiguration,
) (*model.SSOConfiguration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyConfig := cfg
	r.cfg = &copyConfig
	return &copyConfig, nil
}

func TestSSORuntimeKeepsOldSnapshotWhenReplacementIsInvalid(t *testing.T) {
	repo := &memorySSOConfigRepository{}
	runtime := service.NewSSORuntime(repo, config.AuthConfig{}, http.DefaultClient)
	require.NoError(t, runtime.Load(context.Background()))
	before := runtime.Snapshot()

	invalid := model.SSOConfiguration{
		OIDC: model.OIDCSettings{
			Enabled:      true,
			Issuer:       "://bad",
			ClientID:     "sparkops",
			ClientSecret: "secret",
			RedirectURL:  "https://sparkops.example.com/callback",
		},
	}
	invalid.OIDCTestedFingerprint = invalid.ProviderFingerprint(model.SSOProviderOIDC)

	require.Error(t, runtime.Replace(invalid))
	require.Equal(t, before.Configuration.Version, runtime.Snapshot().Configuration.Version)
	require.Nil(t, runtime.Snapshot().OIDC)
}

func TestSSORuntimeRefreshesOnlyWhenRepositoryVersionChanges(t *testing.T) {
	first := model.SSOConfiguration{Version: 1}
	repo := &memorySSOConfigRepository{cfg: &first}
	runtime := service.NewSSORuntime(repo, config.AuthConfig{}, http.DefaultClient)
	require.NoError(t, runtime.Load(context.Background()))
	require.Equal(t, int64(1), runtime.Snapshot().Configuration.Version)

	second := model.SSOConfiguration{Version: 2}
	repo.cfg = &second
	require.NoError(t, runtime.RefreshIfChanged(context.Background()))
	require.Equal(t, int64(2), runtime.Snapshot().Configuration.Version)
}

func TestSSORuntimeValidateDoesNotActivateConfiguration(t *testing.T) {
	repo := &memorySSOConfigRepository{}
	runtime := service.NewSSORuntime(repo, config.AuthConfig{}, http.DefaultClient)
	require.NoError(t, runtime.Load(context.Background()))

	candidate := model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Provider:          model.SSOProviderOIDC,
			ShowPasswordLogin: true,
		},
		Version: 9,
	}
	require.NoError(t, runtime.Validate(candidate))
	require.Equal(t, int64(0), runtime.Snapshot().Configuration.Version)
}

func TestSSORuntimeOIDCTestUsesDiscovery(t *testing.T) {
	var discoveryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/.well-known/openid-configuration", r.URL.Path)
		discoveryCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 serverURL(r),
			"authorization_endpoint": serverURL(r) + "/oauth/authorize",
			"token_endpoint":         serverURL(r) + "/oauth/token",
			"userinfo_endpoint":      serverURL(r) + "/oauth/userinfo",
		})
	}))
	t.Cleanup(server.Close)

	cfg := model.SSOConfiguration{
		OIDC: model.OIDCSettings{
			Issuer:       server.URL,
			ClientID:     "sparkops",
			ClientSecret: "secret",
			RedirectURL:  "https://sparkops.example.com/callback",
			Scopes:       "openid email profile",
		},
	}
	runtime := service.NewSSORuntime(&memorySSOConfigRepository{}, config.AuthConfig{}, server.Client())
	result, err := runtime.TestProvider(context.Background(), cfg, model.SSOProviderOIDC)

	require.NoError(t, err)
	require.Equal(t, cfg.Normalize().ProviderFingerprint(model.SSOProviderOIDC), result.Fingerprint)
	require.Equal(t, int32(1), discoveryCalls.Load())
}

func TestSSORuntimeOIDCTestStillUsesDiscoveryWithEndpointOverrides(t *testing.T) {
	var discoveryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discoveryCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 serverURL(r),
			"authorization_endpoint": serverURL(r) + "/oauth/authorize",
			"token_endpoint":         serverURL(r) + "/oauth/token",
			"userinfo_endpoint":      serverURL(r) + "/oauth/userinfo",
		})
	}))
	t.Cleanup(server.Close)

	cfg := model.SSOConfiguration{
		OIDC: model.OIDCSettings{
			Issuer: server.URL, ClientID: "sparkops", ClientSecret: "secret",
			RedirectURL: "https://sparkops.example.com/callback", Scopes: "openid email profile",
			AuthorizationEndpoint: server.URL + "/custom/authorize",
			TokenEndpoint:         server.URL + "/custom/token",
			UserInfoEndpoint:      server.URL + "/custom/userinfo",
		},
	}
	runtime := service.NewSSORuntime(&memorySSOConfigRepository{}, config.AuthConfig{}, server.Client())
	_, err := runtime.TestProvider(context.Background(), cfg, model.SSOProviderOIDC)

	require.NoError(t, err)
	require.Equal(t, int32(1), discoveryCalls.Load())
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
