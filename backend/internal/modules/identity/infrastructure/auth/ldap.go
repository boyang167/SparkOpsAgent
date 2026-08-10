package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type LDAPUser struct {
	Subject     string
	Email       string
	DisplayName string
}

type LDAPAuthenticator struct {
	cfg config.LDAPConfig
}

func NewLDAPAuthenticator(cfg config.LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg}
}

func (a *LDAPAuthenticator) Authenticate(username, password string) (*LDAPUser, error) {
	if !a.cfg.Enabled {
		return nil, fmt.Errorf("ldap auth disabled")
	}
	conn, err := ldap.DialURL(a.cfg.URL)
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

func RandomPassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
