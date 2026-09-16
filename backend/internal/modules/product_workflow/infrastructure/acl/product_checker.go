package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker struct{ db *gorm.DB }

func NewProductChecker(db *gorm.DB) port.ProductChecker {
	return &ProductChecker{db: db}
}

func (a *ProductChecker) ProductWorkspace(ctx context.Context, productID kernel.ProductID) (kernel.WorkspaceID, error) {
	var wsID string
	err := a.db.WithContext(ctx).Table("product.products").
		Select("workspace_id").Where("id = ?", productID.String()).Scan(&wsID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || wsID == "" {
		return kernel.WorkspaceID{}, domain.ErrProductNotInWorkspace
	}
	if err != nil {
		return kernel.WorkspaceID{}, err
	}
	return kernel.ParseWorkspaceID(wsID)
}

func (a *ProductChecker) ProductName(ctx context.Context, productID kernel.ProductID) (string, error) {
	var name string
	err := a.db.WithContext(ctx).Table("product.products").
		Select("name").Where("id = ?", productID.String()).Scan(&name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || name == "" {
		return "", domain.ErrProductNotInWorkspace
	}
	return name, err
}
