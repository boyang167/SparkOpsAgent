package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProcessRepository interface {
	FindDefaultByProduct(ctx context.Context, productID kernel.ProductID) (*model.SprintProcess, error)
	FindByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*model.SprintProcess, error)
	ListByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.SprintProcess, error)
	Save(ctx context.Context, process *model.SprintProcess) error
	Update(ctx context.Context, process *model.SprintProcess) error
	Delete(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) error
}

type WorkspaceTemplateRepository interface {
	List(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceSprintProcessTemplate, error)
	Save(ctx context.Context, tpl *model.WorkspaceSprintProcessTemplate) error
	Update(ctx context.Context, tpl *model.WorkspaceSprintProcessTemplate) error
	FindByID(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) (*model.WorkspaceSprintProcessTemplate, error)
	Delete(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) error
}
