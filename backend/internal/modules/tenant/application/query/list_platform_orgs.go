package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
)

type PlatformOrganizationView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type ListPlatformOrganizationsHandler struct {
	repo repository.OrganizationRepository
}

func NewListPlatformOrganizationsHandler(repo repository.OrganizationRepository) *ListPlatformOrganizationsHandler {
	return &ListPlatformOrganizationsHandler{repo: repo}
}

func (h *ListPlatformOrganizationsHandler) Handle(ctx context.Context) ([]PlatformOrganizationView, error) {
	items, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return toPlatformOrgViews(items), nil
}

func toPlatformOrgViews(items []*model.Organization) []PlatformOrganizationView {
	out := make([]PlatformOrganizationView, 0, len(items))
	for _, org := range items {
		out = append(out, PlatformOrganizationView{
			ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
			Status: org.Status, CreatedAt: org.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out
}
