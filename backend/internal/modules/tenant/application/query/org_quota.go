package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrgQuotaView struct {
	MaxWorkspaces int   `json:"max_workspaces"`
	MaxAgents     int   `json:"max_agents"`
	MaxProjects   int   `json:"max_projects"`
	TokenBudget   int64 `json:"token_budget"`
}

type OrgQuotaUsageView struct {
	Workspaces int `json:"workspaces"`
	Agents     int `json:"agents"`
	Projects   int `json:"projects"`
}

type GetOrgQuotaHandler struct {
	repo repository.OrganizationRepository
}

func NewGetOrgQuotaHandler(repo repository.OrganizationRepository) *GetOrgQuotaHandler {
	return &GetOrgQuotaHandler{repo: repo}
}

func (h *GetOrgQuotaHandler) Handle(ctx context.Context, orgID kernel.OrgID) (*OrgQuotaView, error) {
	org, err := h.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return &OrgQuotaView{
		MaxWorkspaces: org.Quota.MaxWorkspaces,
		MaxAgents:     org.Quota.MaxAgents,
		MaxProjects:   org.Quota.MaxProjects,
		TokenBudget:   org.Quota.TokenBudget,
	}, nil
}

type GetOrgQuotaUsageHandler struct {
	repo repository.OrganizationRepository
}

func NewGetOrgQuotaUsageHandler(repo repository.OrganizationRepository) *GetOrgQuotaUsageHandler {
	return &GetOrgQuotaUsageHandler{repo: repo}
}

func (h *GetOrgQuotaUsageHandler) Handle(ctx context.Context, orgID kernel.OrgID) (*OrgQuotaUsageView, error) {
	if _, err := h.repo.FindByID(ctx, orgID); err != nil {
		return nil, err
	}
	usage, err := h.repo.GetUsage(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return &OrgQuotaUsageView{
		Workspaces: usage.Workspaces,
		Agents:     usage.Agents,
		Projects:   usage.Projects,
	}, nil
}
