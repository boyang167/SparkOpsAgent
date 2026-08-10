package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MyOrgPermissionsHandler struct {
	repo repository.OrganizationRepository
}

func NewMyOrgPermissionsHandler(repo repository.OrganizationRepository) *MyOrgPermissionsHandler {
	return &MyOrgPermissionsHandler{repo: repo}
}

func (h *MyOrgPermissionsHandler) Handle(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) ([]string, error) {
	role, err := h.repo.GetMemberRole(ctx, orgID, memberID)
	if err != nil {
		return nil, err
	}
	return authz.PermissionsForOrgRole(role), nil
}
