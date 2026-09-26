package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker struct{ db *gorm.DB }

func NewProductChecker(db *gorm.DB) port.ProductChecker {
	return &ProductChecker{db: db}
}

func (a *ProductChecker) ProductExists(ctx context.Context, productID kernel.ProductID) error {
	var id string
	err := a.db.WithContext(ctx).Table("product.products").
		Select("id").Where("id = ?", productID.String()).Scan(&id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || id == "" {
		return domain.ErrProductNotFound
	}
	return err
}
