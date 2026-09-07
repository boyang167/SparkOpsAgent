package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPodRestarter struct {
	access *ProjectClusterAccess
	db     *gorm.DB
}

func NewProjectPodRestarter(access *ProjectClusterAccess, db *gorm.DB) *ProjectPodRestarter {
	return &ProjectPodRestarter{access: access, db: db}
}

func (r *ProjectPodRestarter) Restart(
	ctx context.Context,
	projectID kernel.ProjectID,
	_ string,
	pod *model.Pod,
) error {
	if pod == nil {
		return fmt.Errorf("pod is required")
	}
	session, err := r.openSession(ctx, projectID)
	if err != nil {
		return err
	}
	if pod.Namespace != session.Namespace {
		return fmt.Errorf("pod namespace mismatch")
	}
	err = session.Client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete pod %q: %w", pod.Name, err)
	}
	return nil
}

func (r *ProjectPodRestarter) openSession(
	ctx context.Context,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	wsID, err := r.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return r.access.Open(ctx, wsID, projectID)
}

func (r *ProjectPodRestarter) projectWorkspaceID(
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

var _ port.PodRestarter = (*ProjectPodRestarter)(nil)
