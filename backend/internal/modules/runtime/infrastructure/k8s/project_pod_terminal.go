package k8s

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPodTerminal struct {
	access *ProjectClusterAccess
	db     *gorm.DB
}

func NewProjectPodTerminal(access *ProjectClusterAccess, db *gorm.DB) *ProjectPodTerminal {
	return &ProjectPodTerminal{access: access, db: db}
}

func (t *ProjectPodTerminal) StreamLines(ctx context.Context, pod *model.Pod) []string {
	if pod == nil {
		return []string{"# error: pod is required"}
	}
	session, err := t.openSession(ctx, pod.ProjectID)
	if err != nil {
		return []string{"# error: " + err.Error()}
	}
	if pod.Namespace != session.Namespace {
		return []string{"# error: pod namespace mismatch"}
	}
	if session.RESTConfig == nil {
		return []string{"# error: cluster REST config is missing"}
	}
	terminal := NewPodTerminalWithClient(session.Client, session.RESTConfig)
	return terminal.StreamLines(ctx, pod)
}

func (t *ProjectPodTerminal) openSession(
	ctx context.Context,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	wsID, err := t.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return t.access.Open(ctx, wsID, projectID)
}

func (t *ProjectPodTerminal) projectWorkspaceID(
	ctx context.Context,
	projectID kernel.ProjectID,
) (kernel.WorkspaceID, error) {
	var row struct {
		WorkspaceID string
	}
	err := t.db.WithContext(ctx).Table("workspace_app.projects").
		Select("workspace_id").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if err != nil {
		return kernel.WorkspaceID{}, err
	}
	return kernel.ParseWorkspaceID(row.WorkspaceID)
}

var _ port.PodTerminal = (*ProjectPodTerminal)(nil)
