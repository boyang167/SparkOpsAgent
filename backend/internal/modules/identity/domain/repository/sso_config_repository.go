package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
)

type SSOConfigRepository interface {
	Load(ctx context.Context) (*model.SSOConfiguration, error)
	Save(ctx context.Context, expectedVersion int64, cfg model.SSOConfiguration) (*model.SSOConfiguration, error)
}
