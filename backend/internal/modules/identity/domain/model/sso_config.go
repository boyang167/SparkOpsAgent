package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type SSOProvider string

const (
	SSOProviderOIDC SSOProvider = "oidc"
	SSOProviderLDAP SSOProvider = "ldap"
)

type LoginUISettings struct {
	Enabled           bool        `json:"enabled"`
	Provider          SSOProvider `json:"provider"`
	ShowPasswordLogin bool        `json:"show_password_login"`
}

type OIDCSettings struct {
	Enabled               bool   `json:"enabled"`
	Issuer                string `json:"issuer"`
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"-"`
	RedirectURL           string `json:"redirect_url"`
	Scopes                string `json:"scopes"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type LDAPSettings struct {
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url"`
	BindDN           string `json:"bind_dn"`
	BindPassword     string `json:"-"`
	UserSearchBase   string `json:"user_search_base"`
	UserSearchFilter string `json:"user_search_filter"`
	EmailAttribute   string `json:"email_attribute"`
	NameAttribute    string `json:"name_attribute"`
}

type SSOConfiguration struct {
	LoginUI               LoginUISettings
	OIDC                  OIDCSettings
	LDAP                  LDAPSettings
	OIDCTestedFingerprint string
	LDAPTestedFingerprint string
	Version               int64
}

func (c SSOConfiguration) Normalize() SSOConfiguration {
	c.LoginUI.Provider = SSOProvider(strings.ToLower(strings.TrimSpace(string(c.LoginUI.Provider))))
	if c.LoginUI.Provider == "" {
		c.LoginUI.Provider = SSOProviderOIDC
	}

	c.OIDC.Issuer = strings.TrimRight(strings.TrimSpace(c.OIDC.Issuer), "/")
	c.OIDC.ClientID = strings.TrimSpace(c.OIDC.ClientID)
	c.OIDC.ClientSecret = strings.TrimSpace(c.OIDC.ClientSecret)
	c.OIDC.RedirectURL = strings.TrimSpace(c.OIDC.RedirectURL)
	c.OIDC.Scopes = normalizeScopes(c.OIDC.Scopes)
	c.OIDC.AuthorizationEndpoint = strings.TrimSpace(c.OIDC.AuthorizationEndpoint)
	c.OIDC.TokenEndpoint = strings.TrimSpace(c.OIDC.TokenEndpoint)
	c.OIDC.UserInfoEndpoint = strings.TrimSpace(c.OIDC.UserInfoEndpoint)

	c.LDAP.URL = strings.TrimRight(strings.TrimSpace(c.LDAP.URL), "/")
	c.LDAP.BindDN = strings.TrimSpace(c.LDAP.BindDN)
	c.LDAP.BindPassword = strings.TrimSpace(c.LDAP.BindPassword)
	c.LDAP.UserSearchBase = strings.TrimSpace(c.LDAP.UserSearchBase)
	c.LDAP.UserSearchFilter = strings.TrimSpace(c.LDAP.UserSearchFilter)
	c.LDAP.EmailAttribute = strings.TrimSpace(c.LDAP.EmailAttribute)
	c.LDAP.NameAttribute = strings.TrimSpace(c.LDAP.NameAttribute)
	if c.LDAP.UserSearchFilter == "" {
		c.LDAP.UserSearchFilter = "(uid=%s)"
	}
	if c.LDAP.EmailAttribute == "" {
		c.LDAP.EmailAttribute = "mail"
	}
	if c.LDAP.NameAttribute == "" {
		c.LDAP.NameAttribute = "cn"
	}
	return c
}

func normalizeScopes(raw string) string {
	seen := make(map[string]struct{})
	for _, scope := range strings.Fields(raw) {
		seen[scope] = struct{}{}
	}
	if len(seen) == 0 {
		return "openid email profile"
	}

	out := make([]string, 0, len(seen))
	for _, standard := range []string{"openid", "email", "profile"} {
		if _, ok := seen[standard]; ok {
			out = append(out, standard)
			delete(seen, standard)
		}
	}
	extras := make([]string, 0, len(seen))
	for scope := range seen {
		extras = append(extras, scope)
	}
	sort.Strings(extras)
	return strings.Join(append(out, extras...), " ")
}

func (c SSOConfiguration) ProviderFingerprint(provider SSOProvider) string {
	c = c.Normalize()
	var value any
	switch provider {
	case SSOProviderOIDC:
		value = struct {
			Issuer                string `json:"issuer"`
			ClientID              string `json:"client_id"`
			ClientSecret          string `json:"client_secret"`
			RedirectURL           string `json:"redirect_url"`
			Scopes                string `json:"scopes"`
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			TokenEndpoint         string `json:"token_endpoint"`
			UserInfoEndpoint      string `json:"userinfo_endpoint"`
		}{
			c.OIDC.Issuer, c.OIDC.ClientID, c.OIDC.ClientSecret, c.OIDC.RedirectURL,
			c.OIDC.Scopes, c.OIDC.AuthorizationEndpoint, c.OIDC.TokenEndpoint, c.OIDC.UserInfoEndpoint,
		}
	case SSOProviderLDAP:
		value = struct {
			URL              string `json:"url"`
			BindDN           string `json:"bind_dn"`
			BindPassword     string `json:"bind_password"`
			UserSearchBase   string `json:"user_search_base"`
			UserSearchFilter string `json:"user_search_filter"`
			EmailAttribute   string `json:"email_attribute"`
			NameAttribute    string `json:"name_attribute"`
		}{
			c.LDAP.URL, c.LDAP.BindDN, c.LDAP.BindPassword, c.LDAP.UserSearchBase,
			c.LDAP.UserSearchFilter, c.LDAP.EmailAttribute, c.LDAP.NameAttribute,
		}
	default:
		return ""
	}
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (c SSOConfiguration) ValidateForSave() error {
	c = c.Normalize()
	if c.LoginUI.Provider != SSOProviderOIDC && c.LoginUI.Provider != SSOProviderLDAP {
		return fmt.Errorf("不支持的默认 SSO 提供商")
	}
	if !c.LoginUI.ShowPasswordLogin && (!c.LoginUI.Enabled || (!c.OIDC.Enabled && !c.LDAP.Enabled)) {
		return fmt.Errorf("关闭密码登录前至少启用一个 SSO 提供商")
	}
	if c.LoginUI.Enabled {
		if c.LoginUI.Provider == SSOProviderOIDC && !c.OIDC.Enabled {
			return fmt.Errorf("默认 OIDC 提供商未启用")
		}
		if c.LoginUI.Provider == SSOProviderLDAP && !c.LDAP.Enabled {
			return fmt.Errorf("默认 LDAP 提供商未启用")
		}
	}
	if c.OIDC.Enabled {
		if c.OIDC.Issuer == "" || c.OIDC.ClientID == "" || c.OIDC.ClientSecret == "" || c.OIDC.RedirectURL == "" {
			return fmt.Errorf("OIDC 配置不完整")
		}
		if c.OIDCTestedFingerprint != c.ProviderFingerprint(SSOProviderOIDC) {
			return fmt.Errorf("OIDC 配置需要先测试连接")
		}
	}
	if c.LDAP.Enabled {
		if c.LDAP.URL == "" || c.LDAP.UserSearchBase == "" {
			return fmt.Errorf("LDAP 配置不完整")
		}
		if c.LDAPTestedFingerprint != c.ProviderFingerprint(SSOProviderLDAP) {
			return fmt.Errorf("LDAP 配置需要先测试连接")
		}
	}
	return nil
}
