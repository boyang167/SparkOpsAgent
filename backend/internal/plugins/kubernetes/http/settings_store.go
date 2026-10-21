package http

import (
	"context"
	"errors"
	"strings"

	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type settingsStore struct {
	db *gorm.DB
}

func newSettingsStore(db *gorm.DB) settingsStore {
	return settingsStore{db: db}
}

func (s settingsStore) List(ctx context.Context, wsID kernel.WorkspaceID, clusterIDs []string) (map[string]k8sschema.ClusterSettings, error) {
	if len(clusterIDs) == 0 {
		return map[string]k8sschema.ClusterSettings{}, nil
	}
	var rows []k8sschema.ClusterSettings
	if err := s.db.WithContext(ctx).Where("workspace_id = ? AND cluster_id IN ?", wsID.String(), clusterIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]k8sschema.ClusterSettings, len(rows))
	for _, row := range rows {
		out[row.ClusterID] = row
	}
	return out, nil
}

func (s settingsStore) Upsert(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, req clusterSettingsRequest) (k8sschema.ClusterSettings, error) {
	var saved k8sschema.ClusterSettings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := k8sschema.ClusterSettings{}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&settings, "workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = k8sschema.ClusterSettings{WorkspaceID: wsID.String(), ClusterID: clusterID.String()}
			applyRequest(&settings, req)
			if err := tx.Create(&settings).Error; err != nil {
				return mapSettingsConflict(err)
			}
			saved = settings
			return nil
		}
		if err != nil {
			return err
		}
		applyRequest(&settings, req)
		if err := tx.Save(&settings).Error; err != nil {
			return err
		}
		saved = settings
		return nil
	})
	if err != nil {
		return k8sschema.ClusterSettings{}, err
	}
	return saved, nil
}

func mapSettingsConflict(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return kernel.NewDomainError("PLUGIN_CLUSTER_SETTINGS_CONFLICT", "cluster settings were updated concurrently")
	}
	return err
}
