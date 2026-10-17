package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/stretchr/testify/require"
)

func validSSOConfiguration() model.SSOConfiguration {
	cfg := model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Enabled:           true,
			Provider:          model.SSOProviderOIDC,
			ShowPasswordLogin: true,
		},
		OIDC: model.OIDCSettings{
			Enabled:      true,
			Issuer:       " https://idp.example.com/ ",
			ClientID:     " sparkops ",
			ClientSecret: "secret",
			RedirectURL:  " https://sparkops.example.com/callback ",
			Scopes:       "profile openid email openid",
		},
		LDAP: model.LDAPSettings{
			URL:              " ldaps://ldap.example.com ",
			BindDN:           " cn=reader,dc=example,dc=com ",
			BindPassword:     "bind-secret",
			UserSearchBase:   " ou=people,dc=example,dc=com ",
			UserSearchFilter: "(uid=%s)",
			EmailAttribute:   "mail",
			NameAttribute:    "cn",
		},
	}
	cfg = cfg.Normalize()
	cfg.OIDCTestedFingerprint = cfg.ProviderFingerprint(model.SSOProviderOIDC)
	return cfg
}

func TestSSOConfigurationNormalizeAppliesCanonicalValues(t *testing.T) {
	cfg := validSSOConfiguration()

	require.Equal(t, "https://idp.example.com", cfg.OIDC.Issuer)
	require.Equal(t, "sparkops", cfg.OIDC.ClientID)
	require.Equal(t, "openid email profile", cfg.OIDC.Scopes)
	require.Equal(t, "ldaps://ldap.example.com", cfg.LDAP.URL)
	require.Equal(t, model.SSOProviderOIDC, cfg.LoginUI.Provider)
}

func TestSSOConfigurationFingerprintChangesOnlyForProviderInputs(t *testing.T) {
	cfg := validSSOConfiguration()
	first := cfg.ProviderFingerprint(model.SSOProviderOIDC)

	cfg.LoginUI.ShowPasswordLogin = !cfg.LoginUI.ShowPasswordLogin
	require.Equal(t, first, cfg.ProviderFingerprint(model.SSOProviderOIDC))

	cfg.OIDC.Issuer = "https://idp.example.com/tenant"
	require.NotEqual(t, first, cfg.ProviderFingerprint(model.SSOProviderOIDC))
}

func TestSSOConfigurationFingerprintChangesWhenSecretChanges(t *testing.T) {
	cfg := validSSOConfiguration()
	first := cfg.ProviderFingerprint(model.SSOProviderOIDC)

	cfg.OIDC.ClientSecret = "replacement"
	require.NotEqual(t, first, cfg.ProviderFingerprint(model.SSOProviderOIDC))
}

func TestSSOConfigurationLDAPFingerprintChangesWhenSecretChanges(t *testing.T) {
	cfg := validSSOConfiguration()
	first := cfg.ProviderFingerprint(model.SSOProviderLDAP)

	cfg.LDAP.BindPassword = "replacement"
	require.NotEqual(t, first, cfg.ProviderFingerprint(model.SSOProviderLDAP))
}

func TestSSOConfigurationRejectsPasswordLockout(t *testing.T) {
	cfg := validSSOConfiguration()
	cfg.LoginUI.ShowPasswordLogin = false
	cfg.OIDC.Enabled = false
	cfg.LDAP.Enabled = false

	require.ErrorContains(t, cfg.ValidateForSave(), "至少启用一个")
}

func TestSSOConfigurationRejectsUntestedEnabledProvider(t *testing.T) {
	cfg := validSSOConfiguration()
	cfg.OIDCTestedFingerprint = ""

	require.ErrorContains(t, cfg.ValidateForSave(), "测试连接")
}

func TestSSOConfigurationRejectsDisabledDefaultProvider(t *testing.T) {
	cfg := validSSOConfiguration()
	cfg.LoginUI.Provider = model.SSOProviderLDAP

	require.ErrorContains(t, cfg.ValidateForSave(), "默认")
}
