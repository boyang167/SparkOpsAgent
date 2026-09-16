package acl

import (
	"context"
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type EnvironmentResolver struct{ db *gorm.DB }

func NewEnvironmentResolver(db *gorm.DB) *EnvironmentResolver {
	return &EnvironmentResolver{db: db}
}

type environmentRow struct {
	ID          string
	ProductID   string
	WorkspaceID string
	ClusterID   string
	Namespace   string
	Status      string
}

func (r *EnvironmentResolver) Resolve(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	envID kernel.EnvironmentID,
) (deployport.EnvironmentDeployTarget, error) {
	var env environmentRow
	err := r.db.WithContext(ctx).Table("product.environments").
		Select("id, product_id, workspace_id, cluster_id, namespace, status").
		Where("id = ? AND deleted_at IS NULL", envID.String()).
		First(&env).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentNotFound
	}
	if err != nil {
		return deployport.EnvironmentDeployTarget{}, err
	}
	if env.WorkspaceID != wsID.String() {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentNotFound
	}
	if env.Status != "active" {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentNotFound
	}
	var productID *string
	if err := r.db.WithContext(ctx).Table("workspace_app.projects").
		Select("product_id").Where("id = ?", projectID.String()).Scan(&productID).Error; err != nil {
		return deployport.EnvironmentDeployTarget{}, err
	}
	if productID == nil || *productID == "" || *productID != env.ProductID {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentProductMismatch
	}
	clusterID, err := kernel.ParseClusterInfoID(env.ClusterID)
	if err != nil {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentNotFound
	}
	parsedEnvID, err := kernel.ParseEnvironmentID(env.ID)
	if err != nil {
		return deployport.EnvironmentDeployTarget{}, err
	}
	parsedProductID, err := kernel.ParseProductID(env.ProductID)
	if err != nil {
		return deployport.EnvironmentDeployTarget{}, err
	}
	return deployport.EnvironmentDeployTarget{
		EnvironmentID: parsedEnvID,
		ProductID:     parsedProductID,
		ClusterID:     clusterID,
		Namespace:     env.Namespace,
	}, nil
}
