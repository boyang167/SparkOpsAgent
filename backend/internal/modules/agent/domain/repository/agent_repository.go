package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AgentRepository interface {
	Save(ctx context.Context, agent *model.Agent) error
	Update(ctx context.Context, agent *model.Agent) error
	FindByID(ctx context.Context, id kernel.AgentID) (*model.Agent, error)
	FindByProjectAndName(ctx context.Context, projectID kernel.ProjectID, name model.AgentName) (*model.Agent, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]*model.Agent, error)
	CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
}
