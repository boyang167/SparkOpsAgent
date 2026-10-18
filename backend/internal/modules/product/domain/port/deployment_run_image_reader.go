package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeploymentRunImage struct {
	Repository string
	Tag        string
}

type DeploymentRunImageReader interface {
	FindByRunIDs(ctx context.Context, ids []kernel.DeploymentRunID) (map[string]DeploymentRunImage, error)
}
