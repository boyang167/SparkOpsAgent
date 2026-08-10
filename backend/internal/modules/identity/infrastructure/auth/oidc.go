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
	"time"

	"golang.org/x/oauth2"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

const OIDCProvider = "oidc"

type OIDCUser struct {
	Subject     string
	Email       string
	DisplayName string
}

type OIDCClient struct {
	cfg  config.OIDCConfig
	oauth *oauth2.Config
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
	}
}

func (c *OIDCClient) Enabled() bool { return c.cfg.Enabled && c.cfg.Issuer != "" && c.cfg.ClientID != "" }

func (c *OIDCClient) AuthURL(state string) string {
	return c.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (c *OIDCClient) ExchangeUser(ctx context.Context, code string) (*OIDCUser, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("oidc auth disabled")
	}
	token, err := c.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userInfoURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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

func (c *OIDCClient) userInfoURL() string {
	if c.cfg.UserInfoURL != "" {
		return c.cfg.UserInfoURL
	}
	return strings.TrimSuffix(c.cfg.Issuer, "/") + "/userinfo"
}
