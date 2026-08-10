package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectName(ctx context.Context, projectID kernel.ProjectID) (string, error)
	ProjectNamespace(ctx context.Context, projectID kernel.ProjectID, projectName string) (string, error)
}

type PodLister interface {
	ListByProject(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]*model.Pod, error)
	FindByID(ctx context.Context, projectID kernel.ProjectID, projectName string, podID kernel.PodID) (*model.Pod, error)
}

type PodLogReader interface {
	ReadLogs(ctx context.Context, pod *model.Pod) (string, error)
}

type PodTerminal interface {
	StreamLines(ctx context.Context, pod *model.Pod) []string
}
