package entitlements

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type loadedPlatformConfig struct {
	features           map[string]bool
	license            licenseConfig
	workspaceOverrides map[string]map[string]bool
}

func loadPlatformConfig(ctx context.Context, db *gorm.DB) (loadedPlatformConfig, error) {
	out := loadedPlatformConfig{
		features:           map[string]bool{},
		license:            licenseConfig{Edition: "community"},
		workspaceOverrides: map[string]map[string]bool{},
	}
	if db == nil {
		return out, nil
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
		return out, err
	}
	if row.Config == "" {
		return out, nil
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(row.Config), &cfg); err != nil {
		return out, nil
	}

	if raw, ok := cfg["features"].(map[string]any); ok {
		for key, value := range raw {
			out.features[key] = asBool(value)
		}
	}
	out.license = parseLicenseConfig(cfg["license"])
	out.workspaceOverrides = parseWorkspaceFeatureOverrides(cfg["workspace_features"])
	return out, nil
}

func ResolveForWorkspace(ctx context.Context, db *gorm.DB, workspaceID string) (WorkspaceEntitlements, error) {
	ents := Default()
	cfg, err := loadPlatformConfig(ctx, db)
	if err != nil {
		return ents, err
	}

	applyFeatureMap(&ents, cfg.features)
	applyLicense(&ents, cfg.license, time.Now().UTC())

	clusterCount, err := countClusters(ctx, db)
	if err != nil {
		return ents, err
	}
	if _, explicit := cfg.features["k8s"]; !explicit && clusterCount > 0 {
		ents.K8s = true
	}

	if workspaceID != "" {
		if overrides, ok := cfg.workspaceOverrides[workspaceID]; ok {
			applyWorkspaceOverrides(&ents, overrides)
		}
	}

	return ents, nil
}

func loadFeatureFlags(ctx context.Context, db *gorm.DB) (map[string]bool, error) {
	cfg, err := loadPlatformConfig(ctx, db)
	if err != nil {
		return nil, err
	}
	return cfg.features, nil
}

func Resolve(ctx context.Context, db *gorm.DB) (WorkspaceEntitlements, error) {
	return ResolveForWorkspace(ctx, db, "")
}
