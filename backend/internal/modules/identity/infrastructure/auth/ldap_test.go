package auth_test

import (
	"context"
	"testing"

	"github.com/go-ldap/ldap/v3"
	idauth "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/auth"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type fakeLDAPConnection struct {
	bindUsername string
	bindPassword string
	closed       bool
}

func (c *fakeLDAPConnection) Bind(username, password string) error {
	c.bindUsername = username
	c.bindPassword = password
	return nil
}

func (c *fakeLDAPConnection) Search(*ldap.SearchRequest) (*ldap.SearchResult, error) {
	return &ldap.SearchResult{}, nil
}

func (c *fakeLDAPConnection) Close() error {
	c.closed = true
	return nil
}

type fakeLDAPDialer struct {
	rawURL string
	conn   *fakeLDAPConnection
}

func (d *fakeLDAPDialer) Dial(_ context.Context, stringURL string) (idauth.LDAPConnection, error) {
	d.rawURL = stringURL
	return d.conn, nil
}

func TestLDAPTestConnectionBindsServiceAccount(t *testing.T) {
	conn := &fakeLDAPConnection{}
	dialer := &fakeLDAPDialer{conn: conn}
	authenticator := idauth.NewLDAPAuthenticatorWithDialer(config.LDAPConfig{
		Enabled: true, URL: "ldaps://ldap.example.com",
		BindDN: "cn=reader,dc=example,dc=com", BindPassword: "secret",
		UserSearchBase: "ou=people,dc=example,dc=com", UserSearchFilter: "(uid=%s)",
	}, dialer)

	require.NoError(t, authenticator.TestConnection(context.Background()))
	require.Equal(t, "ldaps://ldap.example.com", dialer.rawURL)
	require.Equal(t, "cn=reader,dc=example,dc=com", conn.bindUsername)
	require.Equal(t, "secret", conn.bindPassword)
	require.True(t, conn.closed)
}
