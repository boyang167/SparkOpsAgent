package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TemplateRepository interface {
	Save(ctx context.Context, template *model.Template) error
	ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Template, error)
}
