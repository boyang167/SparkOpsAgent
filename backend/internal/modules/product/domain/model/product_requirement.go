package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductRequirementStatus string

const (
	ProductRequirementStatusPending   ProductRequirementStatus = "pending"
	ProductRequirementStatusApproved  ProductRequirementStatus = "approved"
	ProductRequirementStatusRejected  ProductRequirementStatus = "rejected"
	ProductRequirementStatusRunning   ProductRequirementStatus = "running"
	ProductRequirementStatusCompleted ProductRequirementStatus = "completed"
)

type ProductRequirement struct {
	ID                  kernel.ProductRequirementID
	ProductID           kernel.ProductID
	ProductPlanID       kernel.ProductPlanID
	ProjectID           kernel.ProjectID
	ProjectName         string
	ProjectTag          string
	BuildID             kernel.BuildID
	Description         string
	Status              ProductRequirementStatus
	MulticlusterStatus  string
	Operator            string
	OtherInfo           map[string]any
	CreatedAt           time.Time
}

func NewProductRequirement(productID kernel.ProductID, planID kernel.ProductPlanID, projectID kernel.ProjectID, projectName, projectTag, description, operator string, otherInfo map[string]any) *ProductRequirement {
	if otherInfo == nil {
		otherInfo = map[string]any{}
	}
	return &ProductRequirement{
		ID:                 kernel.NewProductRequirementID(),
		ProductID:          productID,
		ProductPlanID:      planID,
		ProjectID:          projectID,
		ProjectName:        strings.TrimSpace(projectName),
		ProjectTag:         strings.TrimSpace(projectTag),
		Description:        strings.TrimSpace(description),
		Status:             ProductRequirementStatusPending,
		Operator:           strings.TrimSpace(operator),
		OtherInfo:          otherInfo,
		CreatedAt:          time.Now().UTC(),
	}
}
