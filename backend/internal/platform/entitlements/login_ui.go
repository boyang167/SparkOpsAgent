package entitlements

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
)

type LoginUIConfig struct {
	Enabled           bool   `json:"enabled"`
	Provider          string `json:"provider"`
	ShowPasswordLogin bool   `json:"show_password_login"`
}

func LoadLoginUIConfig(ctx context.Context, db *gorm.DB) (LoginUIConfig, error) {
	out := LoginUIConfig{
		Provider:          "oidc",
		ShowPasswordLogin: true,
	}
	raw, err := loadRawSysConfig(ctx, db)
	if err != nil {
		return out, err
	}
	sso, ok := raw["sso"].(map[string]any)
	if !ok {
		return out, nil
	}
	out.Enabled = asBool(sso["enabled"])
	if provider, ok := sso["provider"].(string); ok && provider != "" {
		out.Provider = provider
	}
	if v, ok := sso["show_password_login"]; ok {
		out.ShowPasswordLogin = asBool(v)
	}
	return out, nil
}

func loadRawSysConfig(ctx context.Context, db *gorm.DB) (map[string]any, error) {
	if db == nil {
		return map[string]any{}, nil
	}
	var row sysCfgRow
	err := db.WithContext(ctx).
		Table("platform_admin.sys_cfg").
		Select("config").
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Config == "" {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(row.Config), &out); err != nil {
		return map[string]any{}, nil
	}
	return out, nil
}
