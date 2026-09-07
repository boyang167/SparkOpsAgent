package k8s

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPodLogReader struct {
	access *ProjectClusterAccess
	db     *gorm.DB
}

func NewProjectPodLogReader(access *ProjectClusterAccess, db *gorm.DB) *ProjectPodLogReader {
	return &ProjectPodLogReader{access: access, db: db}
}

func (r *ProjectPodLogReader) ReadLogs(ctx context.Context, pod *model.Pod) (string, error) {
	if pod == nil {
		return "", fmt.Errorf("pod is required")
	}
	session, err := r.openSession(ctx, pod.ProjectID)
	if err != nil {
		return "", err
	}
	if pod.Namespace != session.Namespace {
		return "", fmt.Errorf("pod namespace mismatch")
	}
	reader := NewPodLogReaderWithClient(session.Client)
	return reader.ReadLogs(ctx, pod)
}

func (r *ProjectPodLogReader) openSession(
	ctx context.Context,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	wsID, err := r.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return r.access.Open(ctx, wsID, projectID)
}

func (r *ProjectPodLogReader) projectWorkspaceID(
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

var _ port.PodLogReader = (*ProjectPodLogReader)(nil)
