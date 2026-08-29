package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
)

type PlatformOrganizationView struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Slug              string  `json:"slug"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	OwnerID           *string `json:"owner_id,omitempty"`
	OwnerEmail        *string `json:"owner_email,omitempty"`
	OwnerDisplayName  *string `json:"owner_display_name,omitempty"`
}

type ListPlatformOrganizationsHandler struct {
	repo repository.OrganizationRepository
}

func NewListPlatformOrganizationsHandler(repo repository.OrganizationRepository) *ListPlatformOrganizationsHandler {
	return &ListPlatformOrganizationsHandler{repo: repo}
}

func (h *ListPlatformOrganizationsHandler) Handle(ctx context.Context) ([]PlatformOrganizationView, error) {
	items, err := h.repo.ListAllIncludingDeleted(ctx)
	if err != nil {
		return nil, err
	}
	owners, err := h.repo.ListOwners(ctx)
	if err != nil {
		return nil, err
	}
	ownerByOrg := make(map[string]repository.OrgOwner, len(owners))
	for _, owner := range owners {
		ownerByOrg[owner.OrgID.String()] = owner
	}
	return toPlatformOrgViews(items, ownerByOrg), nil
}

func toPlatformOrgViews(items []*model.Organization, ownerByOrg map[string]repository.OrgOwner) []PlatformOrganizationView {
	out := make([]PlatformOrganizationView, 0, len(items))
	for _, org := range items {
		view := PlatformOrganizationView{
			ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
			Status: org.Status, CreatedAt: org.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		if owner, ok := ownerByOrg[org.ID.String()]; ok {
			ownerID := owner.MemberID.String()
			ownerEmail := owner.Email
			ownerName := owner.DisplayName
			view.OwnerID = &ownerID
			view.OwnerEmail = &ownerEmail
			view.OwnerDisplayName = &ownerName
		}
		out = append(out, view)
	}
	return out
}
