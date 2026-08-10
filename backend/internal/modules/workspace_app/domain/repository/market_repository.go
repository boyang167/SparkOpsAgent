package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MarketRepository interface {
	SaveDockerfile(ctx context.Context, df *model.Dockerfile) error
	ListDockerfilesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Dockerfile, error)
}
