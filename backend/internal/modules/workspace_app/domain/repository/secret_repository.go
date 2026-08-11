package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SecretRepository interface {
	Save(ctx context.Context, secret *model.Secret) error
	Update(ctx context.Context, secret *model.Secret) error
	Delete(ctx context.Context, id kernel.SecretID) error
	FindByID(ctx context.Context, wsID kernel.WorkspaceID, id kernel.SecretID) (*model.Secret, error)
	ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Secret, error)
}
