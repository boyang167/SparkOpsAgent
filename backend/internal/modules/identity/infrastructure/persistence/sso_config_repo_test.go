package persistence_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSSOConfigRepository(t *testing.T) (*gorm.DB, *persistence.SSOConfigRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS identity").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform_admin").Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.sys_cfg (
		id TEXT PRIMARY KEY,
		kind TEXT,
		name TEXT,
		whether_share BOOLEAN,
		config TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO platform_admin.sys_cfg
		(id, kind, name, whether_share, config, created_at, updated_at)
		VALUES ('default', 'default', 'default', 0, '{"features":{"delivery":true}}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	box, err := crypto.NewSecretBox([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	repo := persistence.NewSSOConfigRepository(db, box)
	require.NoError(t, repo.AutoMigrate())
	return db, repo
}

func persistedConfiguration() model.SSOConfiguration {
	testedAt := time.Date(2026, time.October, 17, 2, 0, 0, 0, time.UTC)
	return model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Enabled:           true,
			Provider:          model.SSOProviderOIDC,
			ShowPasswordLogin: false,
		},
		OIDC: model.OIDCSettings{
			Enabled:      true,
			Issuer:       "https://idp.example.com",
			ClientID:     "sparkops",
			ClientSecret: "oidc-secret",
			RedirectURL:  "https://sparkops.example.com/callback",
			Scopes:       "openid email profile",
		},
		LDAP: model.LDAPSettings{
			URL:              "ldaps://ldap.example.com",
			BindDN:           "cn=reader,dc=example,dc=com",
			BindPassword:     "ldap-secret",
			UserSearchBase:   "ou=people,dc=example,dc=com",
			UserSearchFilter: "(uid=%s)",
			EmailAttribute:   "mail",
			NameAttribute:    "cn",
		},
		OIDCTestedFingerprint: "oidc-fingerprint",
		LDAPTestedFingerprint: "ldap-fingerprint",
		OIDCTestedAt:          &testedAt,
		LDAPTestedAt:          &testedAt,
	}
}

func TestSSOConfigRepositoryEncryptsSecretsAndPreservesSystemConfig(t *testing.T) {
	db, repo := setupSSOConfigRepository(t)
	ctx := context.Background()

	saved, err := repo.Save(ctx, 0, persistedConfiguration())
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.Version)

	var rows []struct {
		Config     string
		SecretBlob string
	}
	require.NoError(t, db.Table("identity.sso_configs").Find(&rows).Error)
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "oidc-secret")
	require.NotContains(t, string(raw), "ldap-secret")

	loaded, err := repo.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "oidc-secret", loaded.OIDC.ClientSecret)
	require.Equal(t, "ldap-secret", loaded.LDAP.BindPassword)
	require.Equal(t, int64(1), loaded.Version)
	require.Equal(t, saved.OIDCTestedAt, loaded.OIDCTestedAt)

	var rawSysConfig string
	require.NoError(t, db.Table("platform_admin.sys_cfg").Select("config").Scan(&rawSysConfig).Error)
	require.Contains(t, rawSysConfig, `"delivery":true`)
	require.Contains(t, rawSysConfig, `"show_password_login":false`)
	require.NotContains(t, rawSysConfig, "secret")
}

func TestSSOConfigRepositoryRejectsStaleVersion(t *testing.T) {
	_, repo := setupSSOConfigRepository(t)
	ctx := context.Background()
	cfg := persistedConfiguration()

	_, err := repo.Save(ctx, 0, cfg)
	require.NoError(t, err)
	_, err = repo.Save(ctx, 0, cfg)
	require.ErrorContains(t, err, "其他管理员")
}
