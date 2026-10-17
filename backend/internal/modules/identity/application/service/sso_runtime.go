package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	idauth "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/auth"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type ProviderTestResult struct {
	Provider    model.SSOProvider
	Fingerprint string
	TestedAt    time.Time
	Message     string
}

type SSOSnapshot struct {
	Configuration model.SSOConfiguration
	OIDC          *idauth.OIDCClient
	LDAP          *idauth.LDAPAuthenticator
}

type SSORuntime struct {
	repo       repository.SSOConfigRepository
	fallback   config.AuthConfig
	httpClient *http.Client
	snapshot   atomic.Pointer[SSOSnapshot]
	now        func() time.Time
}

func NewSSORuntime(
	repo repository.SSOConfigRepository,
	fallback config.AuthConfig,
	httpClient *http.Client,
) *SSORuntime {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &SSORuntime{
		repo: repo, fallback: fallback, httpClient: httpClient,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (r *SSORuntime) Load(ctx context.Context) error {
	var cfg *model.SSOConfiguration
	var err error
	if r.repo != nil {
		cfg, err = r.repo.Load(ctx)
		if err != nil {
			return err
		}
	}
	if cfg == nil {
		fallback := configurationFromAuthConfig(r.fallback)
		cfg = &fallback
	}
	return r.Replace(*cfg)
}

func (r *SSORuntime) Snapshot() SSOSnapshot {
	current := r.snapshot.Load()
	if current == nil {
		return SSOSnapshot{}
	}
	return *current
}

func (r *SSORuntime) TestProvider(
	ctx context.Context,
	cfg model.SSOConfiguration,
	provider model.SSOProvider,
) (ProviderTestResult, error) {
	cfg = prepareConfiguration(cfg)
	testedAt := r.now()
	switch provider {
	case model.SSOProviderOIDC:
		settings := cfg.OIDC
		settings.Enabled = true
		if err := validateOIDCSettings(settings); err != nil {
			return ProviderTestResult{}, err
		}
		client := idauth.NewOIDCClientFromSettings(settings, r.httpClient)
		if err := client.TestConnection(ctx); err != nil {
			return ProviderTestResult{}, err
		}
		return ProviderTestResult{
			Provider: provider, Fingerprint: cfg.ProviderFingerprint(provider),
			TestedAt: testedAt, Message: "OIDC Discovery 与端点校验成功",
		}, nil
	case model.SSOProviderLDAP:
		settings := cfg.LDAP
		settings.Enabled = true
		if err := validateLDAPSettings(settings); err != nil {
			return ProviderTestResult{}, err
		}
		client := idauth.NewLDAPAuthenticatorFromSettings(settings)
		if err := client.TestConnection(ctx); err != nil {
			return ProviderTestResult{}, err
		}
		return ProviderTestResult{
			Provider: provider, Fingerprint: cfg.ProviderFingerprint(provider),
			TestedAt: testedAt, Message: "LDAP 连接与 Bind 校验成功",
		}, nil
	default:
		return ProviderTestResult{}, fmt.Errorf("unsupported SSO provider %q", provider)
	}
}

func (r *SSORuntime) Replace(cfg model.SSOConfiguration) error {
	next, err := r.buildSnapshot(cfg)
	if err != nil {
		return err
	}
	r.snapshot.Store(next)
	return nil
}

func (r *SSORuntime) Validate(cfg model.SSOConfiguration) error {
	_, err := r.buildSnapshot(cfg)
	return err
}

func (r *SSORuntime) buildSnapshot(cfg model.SSOConfiguration) (*SSOSnapshot, error) {
	cfg = prepareConfiguration(cfg)
	if err := cfg.ValidateForSave(); err != nil {
		return nil, err
	}
	next := &SSOSnapshot{Configuration: cfg}
	if cfg.OIDC.Enabled {
		if err := validateOIDCSettings(cfg.OIDC); err != nil {
			return nil, err
		}
		next.OIDC = idauth.NewOIDCClientFromSettings(cfg.OIDC, r.httpClient)
	}
	if cfg.LDAP.Enabled {
		if err := validateLDAPSettings(cfg.LDAP); err != nil {
			return nil, err
		}
		next.LDAP = idauth.NewLDAPAuthenticatorFromSettings(cfg.LDAP)
	}
	return next, nil
}

func (r *SSORuntime) RefreshIfChanged(ctx context.Context) error {
	if r.repo == nil {
		return nil
	}
	cfg, err := r.repo.Load(ctx)
	if err != nil || cfg == nil {
		return err
	}
	if current := r.snapshot.Load(); current != nil && current.Configuration.Version == cfg.Version {
		return nil
	}
	return r.Replace(*cfg)
}

func prepareConfiguration(cfg model.SSOConfiguration) model.SSOConfiguration {
	if strings.TrimSpace(string(cfg.LoginUI.Provider)) == "" {
		cfg.LoginUI.Provider = model.SSOProviderOIDC
		if !cfg.LoginUI.Enabled {
			cfg.LoginUI.ShowPasswordLogin = true
		}
	}
	return cfg.Normalize()
}

func configurationFromAuthConfig(authConfig config.AuthConfig) model.SSOConfiguration {
	cfg := model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Enabled:  authConfig.OIDC.Enabled || authConfig.LDAP.Enabled,
			Provider: model.SSOProviderOIDC, ShowPasswordLogin: true,
		},
		OIDC: model.OIDCSettings{
			Enabled: authConfig.OIDC.Enabled, Issuer: authConfig.OIDC.Issuer,
			ClientID: authConfig.OIDC.ClientID, ClientSecret: authConfig.OIDC.ClientSecret,
			RedirectURL: authConfig.OIDC.RedirectURL, Scopes: authConfig.OIDC.Scopes,
			AuthorizationEndpoint: authConfig.OIDC.AuthURL, TokenEndpoint: authConfig.OIDC.TokenURL,
			UserInfoEndpoint: authConfig.OIDC.UserInfoURL,
		},
		LDAP: model.LDAPSettings{
			Enabled: authConfig.LDAP.Enabled, URL: authConfig.LDAP.URL, BindDN: authConfig.LDAP.BindDN,
			BindPassword: authConfig.LDAP.BindPassword, UserSearchBase: authConfig.LDAP.UserSearchBase,
			UserSearchFilter: authConfig.LDAP.UserSearchFilter, EmailAttribute: authConfig.LDAP.EmailAttribute,
			NameAttribute: authConfig.LDAP.NameAttribute,
		},
	}
	if !cfg.OIDC.Enabled && cfg.LDAP.Enabled {
		cfg.LoginUI.Provider = model.SSOProviderLDAP
	}
	cfg = cfg.Normalize()
	if cfg.OIDC.Enabled {
		cfg.OIDCTestedFingerprint = cfg.ProviderFingerprint(model.SSOProviderOIDC)
	}
	if cfg.LDAP.Enabled {
		cfg.LDAPTestedFingerprint = cfg.ProviderFingerprint(model.SSOProviderLDAP)
	}
	return cfg
}

func validateOIDCSettings(settings model.OIDCSettings) error {
	if settings.Issuer == "" || settings.ClientID == "" || settings.ClientSecret == "" || settings.RedirectURL == "" {
		return fmt.Errorf("OIDC 配置不完整")
	}
	if err := validateURL(settings.Issuer, "http", "https"); err != nil {
		return fmt.Errorf("OIDC Issuer: %w", err)
	}
	if err := validateURL(settings.RedirectURL, "http", "https"); err != nil {
		return fmt.Errorf("OIDC 回调地址: %w", err)
	}
	for name, raw := range map[string]string{
		"授权端点":        settings.AuthorizationEndpoint,
		"Token 端点":    settings.TokenEndpoint,
		"UserInfo 端点": settings.UserInfoEndpoint,
	} {
		if raw != "" {
			if err := validateURL(raw, "http", "https"); err != nil {
				return fmt.Errorf("OIDC %s: %w", name, err)
			}
		}
	}
	return nil
}

func validateLDAPSettings(settings model.LDAPSettings) error {
	if settings.URL == "" || settings.UserSearchBase == "" || !strings.Contains(settings.UserSearchFilter, "%s") {
		return fmt.Errorf("LDAP 配置不完整")
	}
	return validateURL(settings.URL, "ldap", "ldaps")
}

func validateURL(raw string, schemes ...string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("URL 无效")
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return fmt.Errorf("URL 协议必须是 %s", strings.Join(schemes, " 或 "))
}
