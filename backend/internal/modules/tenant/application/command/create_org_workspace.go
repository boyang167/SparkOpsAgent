package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateOrganizationHandler struct {
	repo  repository.OrganizationRepository
	audit audit.Logger
}

func NewCreateOrganizationHandler(repo repository.OrganizationRepository, auditLogger audit.Logger) *CreateOrganizationHandler {
	return &CreateOrganizationHandler{repo: repo, audit: auditLogger}
}

func (h *CreateOrganizationHandler) Handle(ctx context.Context, ownerID kernel.MemberID, name, slug string) (*model.Organization, error) {
	org := model.NewOrganization(name, slug)
	exists, err := h.repo.ExistsBySlug(ctx, org.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrSlugAlreadyExists
	}
	if err := h.repo.Save(ctx, org); err != nil {
		return nil, err
	}
	if err := h.repo.AddMember(ctx, org.ID, ownerID, domain.OrgRoleOwner); err != nil {
		return nil, err
	}
	logAudit(h.audit, ctx, orgAuditEntry(ownerID, audit.ActionOrgCreate, org.ID.String(), org.ID, map[string]any{
		"name": org.Name, "slug": org.Slug, "mode": "self_service",
	}))
	return org, nil
}

type CreateWorkspaceHandler struct {
	orgRepo repository.OrganizationRepository
	wsRepo  repository.WorkspaceRepository
	bus     eventbus.Bus
}

func NewCreateWorkspaceHandler(
	orgRepo repository.OrganizationRepository,
	wsRepo repository.WorkspaceRepository,
	bus eventbus.Bus,
) *CreateWorkspaceHandler {
	return &CreateWorkspaceHandler{orgRepo: orgRepo, wsRepo: wsRepo, bus: bus}
}

func (h *CreateWorkspaceHandler) Handle(ctx context.Context, orgID kernel.OrgID, ownerID kernel.MemberID, name, slug string) (*model.Workspace, error) {
	org, err := h.orgRepo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	wsCount, err := h.orgRepo.GetUsage(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if err := org.Quota.CanCreateWorkspace(wsCount.Workspaces); err != nil {
		return nil, err
	}
	ws := model.NewWorkspace(orgID, name, slug, model.WorkspaceQuota{})
	exists, err := h.wsRepo.ExistsByOrgSlug(ctx, orgID, ws.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrSlugAlreadyExists
	}
	if err := h.wsRepo.Save(ctx, ws); err != nil {
		return nil, err
	}
	if err := h.wsRepo.AddMember(ctx, ws.ID, ownerID, "owner"); err != nil {
		return nil, err
	}
	_ = h.orgRepo.AddMember(ctx, orgID, ownerID, domain.OrgRoleOwner)
	_ = h.bus.Publish(ctx, integration.NewWorkspaceCreated(ws.ID, orgID, ownerID))
	return ws, nil
}
