package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowTemplateRepository interface {
	Save(ctx context.Context, tpl *model.WorkflowTemplate) error
	Update(ctx context.Context, tpl *model.WorkflowTemplate) error
	FindByID(ctx context.Context, id kernel.WorkflowTemplateID) (*model.WorkflowTemplate, error)
	FindByKey(ctx context.Context, key string) (*model.WorkflowTemplate, error)
	ListForWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.WorkflowTemplate, error)
}
