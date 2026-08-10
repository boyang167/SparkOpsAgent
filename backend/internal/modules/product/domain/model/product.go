package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductName struct {
	value string
}

func NewProductName(raw string) (ProductName, error) {
	v := strings.TrimSpace(raw)
	if len(v) < 3 || len(v) > 100 {
		return ProductName{}, domain.ErrInvalidProductName
	}
	return ProductName{value: v}, nil
}

func (n ProductName) String() string { return n.value }

type Product struct {
	ID          kernel.ProductID
	WorkspaceID kernel.WorkspaceID
	Name        ProductName
	DisplayName string
	Description string
	Version     string
	NextVersion string
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
}

func NewProduct(wsID kernel.WorkspaceID, name ProductName, displayName, description, version, nextVersion string, createdBy kernel.MemberID) *Product {
	return &Product{
		ID:          kernel.NewProductID(),
		WorkspaceID: wsID,
		Name:        name,
		DisplayName: strings.TrimSpace(displayName),
		Description: strings.TrimSpace(description),
		Version:     strings.TrimSpace(version),
		NextVersion: strings.TrimSpace(nextVersion),
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}
}

func (p *Product) Update(displayName, description, version, nextVersion string) {
	p.DisplayName = strings.TrimSpace(displayName)
	p.Description = strings.TrimSpace(description)
	p.Version = strings.TrimSpace(version)
	p.NextVersion = strings.TrimSpace(nextVersion)
}
