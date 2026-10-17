package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type ssoConfigModel struct {
	Provider          string `gorm:"primaryKey;size:16"`
	Enabled           bool
	Config            string `gorm:"type:text;not null"`
	SecretBlob        string `gorm:"type:text"`
	TestedFingerprint string `gorm:"size:64"`
	TestedAt          *time.Time
	Version           int64 `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ssoConfigModel) TableName() string { return "identity.sso_configs" }

type oidcPublicConfig struct {
	Issuer                string `json:"issuer"`
	ClientID              string `json:"client_id"`
	RedirectURL           string `json:"redirect_url"`
	Scopes                string `json:"scopes"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type ldapPublicConfig struct {
	URL              string `json:"url"`
	BindDN           string `json:"bind_dn"`
	UserSearchBase   string `json:"user_search_base"`
	UserSearchFilter string `json:"user_search_filter"`
	EmailAttribute   string `json:"email_attribute"`
	NameAttribute    string `json:"name_attribute"`
}

type providerSecrets struct {
	ClientSecret string `json:"client_secret,omitempty"`
	BindPassword string `json:"bind_password,omitempty"`
}

type sysConfigRecord struct {
	ID     string
	Config string
}

type SSOConfigRepository struct {
	db  *gorm.DB
	box *crypto.SecretBox
}

func NewSSOConfigRepository(db *gorm.DB, box *crypto.SecretBox) *SSOConfigRepository {
	return &SSOConfigRepository{db: db, box: box}
}

func (r *SSOConfigRepository) AutoMigrate() error {
	if !postgres.ShouldRunAutoMigrate(r.db) {
		return nil
	}
	if r.db.Dialector.Name() == "postgres" {
		if err := r.db.Exec("CREATE SCHEMA IF NOT EXISTS identity").Error; err != nil {
			return err
		}
	}
	return r.db.AutoMigrate(&ssoConfigModel{})
}

func (r *SSOConfigRepository) Load(ctx context.Context) (*model.SSOConfiguration, error) {
	var rows []ssoConfigModel
	if err := r.db.WithContext(ctx).Order("provider ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	cfg := model.SSOConfiguration{
		LoginUI: model.LoginUISettings{
			Provider:          model.SSOProviderOIDC,
			ShowPasswordLogin: true,
		},
	}
	for _, row := range rows {
		if row.Version > cfg.Version {
			cfg.Version = row.Version
		}
		plain, err := r.box.Decrypt(row.SecretBlob)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s SSO secrets: %w", row.Provider, err)
		}
		var secrets providerSecrets
		if plain != "" {
			if err := json.Unmarshal([]byte(plain), &secrets); err != nil {
				return nil, fmt.Errorf("decode %s SSO secrets: %w", row.Provider, err)
			}
		}
		switch model.SSOProvider(row.Provider) {
		case model.SSOProviderOIDC:
			var public oidcPublicConfig
			if err := json.Unmarshal([]byte(row.Config), &public); err != nil {
				return nil, err
			}
			cfg.OIDC = model.OIDCSettings{
				Enabled: row.Enabled, Issuer: public.Issuer, ClientID: public.ClientID,
				ClientSecret: secrets.ClientSecret, RedirectURL: public.RedirectURL, Scopes: public.Scopes,
				AuthorizationEndpoint: public.AuthorizationEndpoint, TokenEndpoint: public.TokenEndpoint,
				UserInfoEndpoint: public.UserInfoEndpoint,
			}
			cfg.OIDCTestedFingerprint = row.TestedFingerprint
			cfg.OIDCTestedAt = row.TestedAt
		case model.SSOProviderLDAP:
			var public ldapPublicConfig
			if err := json.Unmarshal([]byte(row.Config), &public); err != nil {
				return nil, err
			}
			cfg.LDAP = model.LDAPSettings{
				Enabled: row.Enabled, URL: public.URL, BindDN: public.BindDN,
				BindPassword: secrets.BindPassword, UserSearchBase: public.UserSearchBase,
				UserSearchFilter: public.UserSearchFilter, EmailAttribute: public.EmailAttribute,
				NameAttribute: public.NameAttribute,
			}
			cfg.LDAPTestedFingerprint = row.TestedFingerprint
			cfg.LDAPTestedAt = row.TestedAt
		}
	}
	if err := r.loadLoginUI(ctx, &cfg); err != nil {
		return nil, err
	}
	normalized := cfg.Normalize()
	return &normalized, nil
}

func (r *SSOConfigRepository) Save(
	ctx context.Context,
	expectedVersion int64,
	cfg model.SSOConfiguration,
) (*model.SSOConfiguration, error) {
	cfg = cfg.Normalize()
	nextVersion := expectedVersion + 1
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []ssoConfigModel
		if err := tx.Find(&rows).Error; err != nil {
			return err
		}
		currentVersion := int64(0)
		for _, row := range rows {
			if row.Version > currentVersion {
				currentVersion = row.Version
			}
		}
		if currentVersion != expectedVersion {
			return domain.ErrSSOVersionConflict
		}

		oidcRow, err := r.oidcRow(cfg, nextVersion)
		if err != nil {
			return err
		}
		ldapRow, err := r.ldapRow(cfg, nextVersion)
		if err != nil {
			return err
		}
		for _, row := range []ssoConfigModel{oidcRow, ldapRow} {
			if expectedVersion == 0 {
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				continue
			}
			result := tx.Model(&ssoConfigModel{}).
				Where("provider = ? AND version = ?", row.Provider, expectedVersion).
				Updates(map[string]any{
					"enabled": row.Enabled, "config": row.Config, "secret_blob": row.SecretBlob,
					"tested_fingerprint": row.TestedFingerprint, "tested_at": row.TestedAt,
					"version": row.Version, "updated_at": row.UpdatedAt,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrSSOVersionConflict
			}
		}
		return r.saveLoginUI(tx, cfg.LoginUI)
	})
	if err != nil {
		return nil, err
	}
	cfg.Version = nextVersion
	return &cfg, nil
}

func (r *SSOConfigRepository) oidcRow(cfg model.SSOConfiguration, version int64) (ssoConfigModel, error) {
	public, err := json.Marshal(oidcPublicConfig{
		Issuer: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID, RedirectURL: cfg.OIDC.RedirectURL,
		Scopes: cfg.OIDC.Scopes, AuthorizationEndpoint: cfg.OIDC.AuthorizationEndpoint,
		TokenEndpoint: cfg.OIDC.TokenEndpoint, UserInfoEndpoint: cfg.OIDC.UserInfoEndpoint,
	})
	if err != nil {
		return ssoConfigModel{}, err
	}
	return r.providerRow(
		model.SSOProviderOIDC, cfg.OIDC.Enabled, public,
		providerSecrets{ClientSecret: cfg.OIDC.ClientSecret},
		cfg.OIDCTestedFingerprint, cfg.OIDCTestedAt, version,
	)
}

func (r *SSOConfigRepository) ldapRow(cfg model.SSOConfiguration, version int64) (ssoConfigModel, error) {
	public, err := json.Marshal(ldapPublicConfig{
		URL: cfg.LDAP.URL, BindDN: cfg.LDAP.BindDN, UserSearchBase: cfg.LDAP.UserSearchBase,
		UserSearchFilter: cfg.LDAP.UserSearchFilter, EmailAttribute: cfg.LDAP.EmailAttribute,
		NameAttribute: cfg.LDAP.NameAttribute,
	})
	if err != nil {
		return ssoConfigModel{}, err
	}
	return r.providerRow(
		model.SSOProviderLDAP, cfg.LDAP.Enabled, public,
		providerSecrets{BindPassword: cfg.LDAP.BindPassword},
		cfg.LDAPTestedFingerprint, cfg.LDAPTestedAt, version,
	)
}

func (r *SSOConfigRepository) providerRow(
	provider model.SSOProvider,
	enabled bool,
	public []byte,
	secrets providerSecrets,
	fingerprint string,
	testedAt *time.Time,
	version int64,
) (ssoConfigModel, error) {
	rawSecrets, err := json.Marshal(secrets)
	if err != nil {
		return ssoConfigModel{}, err
	}
	encrypted, err := r.box.Encrypt(string(rawSecrets))
	if err != nil {
		return ssoConfigModel{}, err
	}
	now := time.Now().UTC()
	return ssoConfigModel{
		Provider: string(provider), Enabled: enabled, Config: string(public), SecretBlob: encrypted,
		TestedFingerprint: fingerprint, TestedAt: testedAt, Version: version,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r *SSOConfigRepository) loadLoginUI(ctx context.Context, cfg *model.SSOConfiguration) error {
	var row sysConfigRecord
	err := r.db.WithContext(ctx).Table("platform_admin.sys_cfg").
		Select("id, config").Where("deleted_at IS NULL").Order("updated_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	raw := map[string]any{}
	if row.Config != "" {
		if err := json.Unmarshal([]byte(row.Config), &raw); err != nil {
			return err
		}
	}
	sso, ok := raw["sso"].(map[string]any)
	if !ok {
		return nil
	}
	cfg.LoginUI.Enabled, _ = sso["enabled"].(bool)
	if provider, ok := sso["provider"].(string); ok {
		cfg.LoginUI.Provider = model.SSOProvider(provider)
	}
	if showPassword, ok := sso["show_password_login"].(bool); ok {
		cfg.LoginUI.ShowPasswordLogin = showPassword
	}
	return nil
}

func (r *SSOConfigRepository) saveLoginUI(tx *gorm.DB, ui model.LoginUISettings) error {
	var row sysConfigRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Table("platform_admin.sys_cfg").
		Select("id, config").Where("deleted_at IS NULL").Order("updated_at DESC").First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	raw := map[string]any{}
	if row.Config != "" {
		if err := json.Unmarshal([]byte(row.Config), &raw); err != nil {
			return err
		}
	}
	raw["sso"] = map[string]any{
		"enabled": ui.Enabled, "provider": string(ui.Provider),
		"show_password_login": ui.ShowPasswordLogin,
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) || row.ID == "" {
		return tx.Table("platform_admin.sys_cfg").Create(map[string]any{
			"id": uuid.NewString(), "kind": "default", "name": "default",
			"whether_share": false, "config": string(payload), "created_at": now, "updated_at": now,
		}).Error
	}
	return tx.Table("platform_admin.sys_cfg").Where("id = ?", row.ID).
		Updates(map[string]any{"config": string(payload), "updated_at": now}).Error
}

var _ repository.SSOConfigRepository = (*SSOConfigRepository)(nil)
