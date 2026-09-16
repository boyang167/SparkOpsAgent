package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker struct{ db *gorm.DB }

func NewProductChecker(db *gorm.DB) port.ProductChecker {
	return &ProductChecker{db: db}
}

func (c *ProductChecker) ProductInWorkspace(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
) (bool, error) {
	var count int64
	err := c.db.WithContext(ctx).
		Table("product.products").
		Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", productID.String(), wsID.String()).
		Count(&count).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
