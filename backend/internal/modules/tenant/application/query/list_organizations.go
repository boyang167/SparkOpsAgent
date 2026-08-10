package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrganizationView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"my_role,omitempty"`
}

type ListOrganizationsHandler struct {
	repo repository.OrganizationRepository
}

func NewListOrganizationsHandler(repo repository.OrganizationRepository) *ListOrganizationsHandler {
	return &ListOrganizationsHandler{repo: repo}
}

func (h *ListOrganizationsHandler) Handle(ctx context.Context, memberID kernel.MemberID) ([]OrganizationView, error) {
	items, err := h.repo.ListByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationView, 0, len(items))
	for _, org := range items {
		out = append(out, OrganizationView{
			ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
		})
	}
	return out, nil
}
