package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type LDAPUser struct {
	Subject     string
	Email       string
	DisplayName string
}

type LDAPAuthenticator struct {
	cfg    config.LDAPConfig
	dialer LDAPDialer
}

type LDAPConnection interface {
	Bind(username, password string) error
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}

type LDAPDialer interface {
	Dial(ctx context.Context, rawURL string) (LDAPConnection, error)
}

type defaultLDAPDialer struct {
	timeout time.Duration
}

func (d defaultLDAPDialer) Dial(ctx context.Context, rawURL string) (LDAPConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ldap.DialURL(rawURL, ldap.DialWithDialer(&net.Dialer{Timeout: d.timeout}))
}

func NewLDAPAuthenticator(cfg config.LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg, dialer: defaultLDAPDialer{timeout: 10 * time.Second}}
}

func NewLDAPAuthenticatorFromSettings(settings model.LDAPSettings) *LDAPAuthenticator {
	return NewLDAPAuthenticator(config.LDAPConfig{
		Enabled: settings.Enabled, URL: settings.URL, BindDN: settings.BindDN,
		BindPassword: settings.BindPassword, UserSearchBase: settings.UserSearchBase,
		UserSearchFilter: settings.UserSearchFilter, EmailAttribute: settings.EmailAttribute,
		NameAttribute: settings.NameAttribute,
	})
}

func NewLDAPAuthenticatorWithDialer(cfg config.LDAPConfig, dialer LDAPDialer) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg, dialer: dialer}
}

func (a *LDAPAuthenticator) Authenticate(username, password string) (*LDAPUser, error) {
	return a.AuthenticateContext(context.Background(), username, password)
}

func (a *LDAPAuthenticator) AuthenticateContext(
	ctx context.Context,
	username string,
	password string,
) (*LDAPUser, error) {
	if !a.cfg.Enabled {
		return nil, fmt.Errorf("ldap auth disabled")
	}
	conn, err := a.dialer.Dial(ctx, a.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if a.cfg.BindDN != "" {
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap bind: %w", err)
		}
	}

	filter := fmt.Sprintf(a.cfg.UserSearchFilter, ldap.EscapeFilter(username))
	searchReq := ldap.NewSearchRequest(
		a.cfg.UserSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		filter,
		[]string{"dn", a.cfg.EmailAttribute, a.cfg.NameAttribute, "uid"},
		nil,
	)
	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("ldap user not found")
	}
	entry := result.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return nil, fmt.Errorf("ldap invalid credentials")
	}
	email := entry.GetAttributeValue(a.cfg.EmailAttribute)
	if email == "" {
		email = username
	}
	name := entry.GetAttributeValue(a.cfg.NameAttribute)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	subject := entry.GetAttributeValue("uid")
	if subject == "" {
		subject = entry.DN
	}
	return &LDAPUser{Subject: subject, Email: strings.ToLower(strings.TrimSpace(email)), DisplayName: name}, nil
}

func (a *LDAPAuthenticator) TestConnection(ctx context.Context) error {
	if !a.cfg.Enabled {
		return fmt.Errorf("ldap auth disabled")
	}
	if a.cfg.URL == "" || a.cfg.UserSearchBase == "" || !strings.Contains(a.cfg.UserSearchFilter, "%s") {
		return fmt.Errorf("ldap configuration incomplete")
	}
	conn, err := a.dialer.Dial(ctx, a.cfg.URL)
	if err != nil {
		return fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()
	if a.cfg.BindDN != "" {
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			return fmt.Errorf("ldap bind: %w", err)
		}
	}
	return nil
}

func RandomPassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
