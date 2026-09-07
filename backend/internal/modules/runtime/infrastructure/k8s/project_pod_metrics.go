package k8s

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPodMetricsReader struct {
	access *ProjectClusterAccess
	db     *gorm.DB
}

func NewProjectPodMetricsReader(access *ProjectClusterAccess, db *gorm.DB) *ProjectPodMetricsReader {
	return &ProjectPodMetricsReader{access: access, db: db}
}

func (r *ProjectPodMetricsReader) MetricsForPods(
	ctx context.Context,
	projectID kernel.ProjectID,
	pods []*model.Pod,
) (map[kernel.PodID]port.PodMetrics, error) {
	if len(pods) == 0 {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	session, err := r.openSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if session.RESTConfig == nil {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	metricsByName, err := fetchPodMetricsByName(ctx, session.RESTConfig, session.Namespace)
	if err != nil {
		return map[kernel.PodID]port.PodMetrics{}, nil
	}
	return metricsForPods(metricsByName, pods), nil
}

func (r *ProjectPodMetricsReader) openSession(
	ctx context.Context,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	wsID, err := r.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return r.access.Open(ctx, wsID, projectID)
}

func (r *ProjectPodMetricsReader) projectWorkspaceID(
	ctx context.Context,
	projectID kernel.ProjectID,
) (kernel.WorkspaceID, error) {
	var row struct {
		WorkspaceID string
	}
	err := r.db.WithContext(ctx).Table("workspace_app.projects").
		Select("workspace_id").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if err != nil {
		return kernel.WorkspaceID{}, err
	}
	return kernel.ParseWorkspaceID(row.WorkspaceID)
}

var _ port.PodMetricsReader = (*ProjectPodMetricsReader)(nil)
