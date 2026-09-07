package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPodLister struct {
	access *ProjectClusterAccess
	db     *gorm.DB
}

func NewProjectPodLister(access *ProjectClusterAccess, db *gorm.DB) *ProjectPodLister {
	return &ProjectPodLister{access: access, db: db}
}

func (l *ProjectPodLister) ListByProject(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName string,
) ([]*model.Pod, error) {
	session, err := l.openSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return listProjectPods(ctx, session.Client, projectID, session.Namespace, projectName)
}

func (l *ProjectPodLister) FindByID(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName string,
	podID kernel.PodID,
) (*model.Pod, error) {
	pods, err := l.ListByProject(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if pod.ID == podID {
			return pod, nil
		}
	}
	return nil, domain.ErrPodNotFound
}

func (l *ProjectPodLister) openSession(
	ctx context.Context,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	wsID, err := l.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return l.access.Open(ctx, wsID, projectID)
}

func (l *ProjectPodLister) projectWorkspaceID(
	ctx context.Context,
	projectID kernel.ProjectID,
) (kernel.WorkspaceID, error) {
	var row struct {
		WorkspaceID string
	}
	err := l.db.WithContext(ctx).Table("workspace_app.projects").
		Select("workspace_id").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if err != nil {
		return kernel.WorkspaceID{}, err
	}
	return kernel.ParseWorkspaceID(row.WorkspaceID)
}

func listProjectPods(
	ctx context.Context,
	client kubernetes.Interface,
	projectID kernel.ProjectID,
	namespace, projectName string,
) ([]*model.Pod, error) {
	items, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", projectName),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
	}
	if len(items.Items) == 0 {
		items, err = client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
		}
	}
	out := make([]*model.Pod, 0, len(items.Items))
	for i := range items.Items {
		out = append(out, MapPod(projectID, items.Items[i]))
	}
	return out, nil
}

var _ port.PodLister = (*ProjectPodLister)(nil)
