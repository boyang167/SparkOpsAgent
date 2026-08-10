package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductPlanStatus string

const (
	ProductPlanStatusDraft     ProductPlanStatus = "draft"
	ProductPlanStatusActive    ProductPlanStatus = "active"
	ProductPlanStatusCompleted ProductPlanStatus = "completed"
)

type ProductPlan struct {
	ID          kernel.ProductPlanID
	ProductID   kernel.ProductID
	Name        string
	Version     string
	Status      ProductPlanStatus
	Description string
	Submitted   bool
	OtherInfo   map[string]any
	CreatedAt   time.Time
}

func NewProductPlan(productID kernel.ProductID, name, version, description string, otherInfo map[string]any) *ProductPlan {
	if otherInfo == nil {
		otherInfo = map[string]any{}
	}
	return &ProductPlan{
		ID:          kernel.NewProductPlanID(),
		ProductID:   productID,
		Name:        strings.TrimSpace(name),
		Version:     strings.TrimSpace(version),
		Status:      ProductPlanStatusDraft,
		Description: strings.TrimSpace(description),
		OtherInfo:   otherInfo,
		CreatedAt:   time.Now().UTC(),
	}
}
