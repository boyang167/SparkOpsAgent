package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

const OIDCProvider = "oidc"

type OIDCUser struct {
	Subject     string
	Email       string
	DisplayName string
}

type OIDCClient struct {
	cfg         config.OIDCConfig
	oauth       *oauth2.Config
	httpClient  *http.Client
	userInfoURL string
	mu          sync.RWMutex
	discovered  bool
}

type OIDCDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

func NewOIDCClient(cfg config.OIDCConfig) *OIDCClient {
	scopes := strings.Fields(cfg.Scopes)
	authURL := cfg.AuthURL
	tokenURL := cfg.TokenURL
	if authURL == "" {
		authURL = strings.TrimSuffix(cfg.Issuer, "/") + "/authorize"
	}
	if tokenURL == "" {
		tokenURL = strings.TrimSuffix(cfg.Issuer, "/") + "/token"
	}
	return &OIDCClient{
		cfg: cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
			Scopes: scopes,
		},
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		userInfoURL: cfg.UserInfoURL,
		discovered:  cfg.AuthURL != "" && cfg.TokenURL != "" && cfg.UserInfoURL != "",
	}
}

func NewOIDCClientFromSettings(settings model.OIDCSettings, client *http.Client) *OIDCClient {
	oidc := NewOIDCClient(config.OIDCConfig{
		Enabled: settings.Enabled, Issuer: settings.Issuer, ClientID: settings.ClientID,
		ClientSecret: settings.ClientSecret, RedirectURL: settings.RedirectURL, Scopes: settings.Scopes,
		AuthURL: settings.AuthorizationEndpoint, TokenURL: settings.TokenEndpoint,
		UserInfoURL: settings.UserInfoEndpoint,
	})
	if client != nil {
		oidc.httpClient = client
	}
	oidc.discovered = false
	return oidc
}

func (c *OIDCClient) Enabled() bool {
	return c.cfg.Enabled && c.cfg.Issuer != "" && c.cfg.ClientID != ""
}

func (c *OIDCClient) AuthURL(state string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (c *OIDCClient) AuthURLContext(ctx context.Context, state string) (string, error) {
	if err := c.ensureDiscovery(ctx); err != nil {
		return "", err
	}
	return c.AuthURL(state), nil
}

func (c *OIDCClient) TestConnection(ctx context.Context) error {
	return c.ensureDiscovery(ctx)
}

func (c *OIDCClient) ExchangeUser(ctx context.Context, code string) (*OIDCUser, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("oidc auth disabled")
	}
	if err := c.ensureDiscovery(ctx); err != nil {
		return nil, err
	}
	c.mu.RLock()
	oauthConfig := *c.oauth
	userInfoURL := c.userInfoURL
	c.mu.RUnlock()
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc userinfo: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc userinfo status %d", resp.StatusCode)
	}
	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, err
	}
	subject, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if email == "" {
		email, _ = claims["preferred_username"].(string)
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	if subject == "" || email == "" {
		return nil, fmt.Errorf("oidc missing subject or email")
	}
	return &OIDCUser{
		Subject: subject, Email: strings.ToLower(strings.TrimSpace(email)), DisplayName: name,
	}, nil
}

func NewOAuthState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ParseOAuthState(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("missing state")
	}
	if _, err := url.QueryUnescape(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func (c *OIDCClient) ensureDiscovery(ctx context.Context) error {
	if !c.Enabled() {
		return fmt.Errorf("oidc auth disabled")
	}
	c.mu.RLock()
	if c.discovered {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	discovery, err := DiscoverOIDC(ctx, c.httpClient, c.cfg.Issuer)
	if err != nil {
		return err
	}
	authURL := discovery.AuthorizationEndpoint
	tokenURL := discovery.TokenEndpoint
	userInfoURL := discovery.UserInfoEndpoint
	if c.cfg.AuthURL != "" {
		authURL = c.cfg.AuthURL
	}
	if c.cfg.TokenURL != "" {
		tokenURL = c.cfg.TokenURL
	}
	if c.cfg.UserInfoURL != "" {
		userInfoURL = c.cfg.UserInfoURL
	}
	for name, value := range map[string]string{
		"authorization endpoint": authURL,
		"token endpoint":         tokenURL,
		"userinfo endpoint":      userInfoURL,
	} {
		if err := validateHTTPURL(value); err != nil {
			return fmt.Errorf("oidc %s: %w", name, err)
		}
	}

	c.mu.Lock()
	c.oauth.Endpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
	c.userInfoURL = userInfoURL
	c.discovered = true
	c.mu.Unlock()
	return nil
}

func DiscoverOIDC(ctx context.Context, client *http.Client, issuer string) (OIDCDiscovery, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if err := validateHTTPURL(issuer); err != nil {
		return OIDCDiscovery{}, fmt.Errorf("oidc issuer: %w", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		issuer+"/.well-known/openid-configuration",
		nil,
	)
	if err != nil {
		return OIDCDiscovery{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return OIDCDiscovery{}, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OIDCDiscovery{}, fmt.Errorf("oidc discovery status %d", resp.StatusCode)
	}
	var discovery OIDCDiscovery
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&discovery); err != nil {
		return OIDCDiscovery{}, fmt.Errorf("oidc discovery decode: %w", err)
	}
	if strings.TrimRight(discovery.Issuer, "/") != issuer {
		return OIDCDiscovery{}, fmt.Errorf("oidc discovery issuer mismatch")
	}
	for name, value := range map[string]string{
		"authorization endpoint": discovery.AuthorizationEndpoint,
		"token endpoint":         discovery.TokenEndpoint,
		"userinfo endpoint":      discovery.UserInfoEndpoint,
	} {
		if err := validateHTTPURL(value); err != nil {
			return OIDCDiscovery{}, fmt.Errorf("oidc %s: %w", name, err)
		}
	}
	return discovery, nil
}

func validateHTTPURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL host is required")
	}
	return nil
}
