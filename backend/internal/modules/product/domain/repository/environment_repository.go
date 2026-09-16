package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentRepository interface {
	SaveEnvironment(ctx context.Context, env *model.Environment) error
	FindEnvironmentByID(ctx context.Context, id kernel.EnvironmentID) (*model.Environment, error)
	ListEnvironmentsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.Environment, error)
	DeleteEnvironment(ctx context.Context, id kernel.EnvironmentID) error
	CountEnvironmentsByProduct(ctx context.Context, productID kernel.ProductID) (int, error)

	SaveEnvironmentProject(ctx context.Context, row *model.EnvironmentProject) error
	ListEnvironmentProjects(ctx context.Context, envID kernel.EnvironmentID) ([]*model.EnvironmentProject, error)
	EnsureEnvironmentProjects(ctx context.Context, envID kernel.EnvironmentID, projectIDs []kernel.ProjectID) error
	FindEnvironmentProject(ctx context.Context, envID kernel.EnvironmentID, projectID kernel.ProjectID) (*model.EnvironmentProject, error)
}
