package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateOrgQuotaInput struct {
	MaxWorkspaces *int
	MaxAgents     *int
	MaxProjects   *int
	TokenBudget   *int64
}

type UpdateOrgQuotaHandler struct {
	repo repository.OrganizationRepository
}

func NewUpdateOrgQuotaHandler(repo repository.OrganizationRepository) *UpdateOrgQuotaHandler {
	return &UpdateOrgQuotaHandler{repo: repo}
}

func (h *UpdateOrgQuotaHandler) Handle(ctx context.Context, orgID kernel.OrgID, in UpdateOrgQuotaInput) (*model.OrgQuota, error) {
	org, err := h.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	quota := org.Quota
	if in.MaxWorkspaces != nil {
		quota.MaxWorkspaces = *in.MaxWorkspaces
	}
	if in.MaxAgents != nil {
		quota.MaxAgents = *in.MaxAgents
	}
	if in.MaxProjects != nil {
		quota.MaxProjects = *in.MaxProjects
	}
	if in.TokenBudget != nil {
		quota.TokenBudget = *in.TokenBudget
	}
	if err := h.repo.UpdateQuota(ctx, orgID, quota); err != nil {
		return nil, err
	}
	return &quota, nil
}
