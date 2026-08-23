package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetOrganizationHandler struct {
	repo repository.OrganizationRepository
}

func NewGetOrganizationHandler(repo repository.OrganizationRepository) *GetOrganizationHandler {
	return &GetOrganizationHandler{repo: repo}
}

func (h *GetOrganizationHandler) Handle(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (*OrganizationView, error) {
	org, err := h.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	role, err := h.repo.GetMemberRole(ctx, orgID, memberID)
	if err != nil {
		return nil, domain.ErrOrgAccessDenied
	}
	return &OrganizationView{
		ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
		MyRole: role, IsMember: true, ReadOnly: false,
	}, nil
}
