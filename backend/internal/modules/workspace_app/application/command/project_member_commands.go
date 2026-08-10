package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AddProjectMemberHandler struct {
	projectRepo repository.ProjectRepository
	memberRepo  repository.ProjectMemberRepository
	wsChecker   port.WorkspaceChecker
	governance  authz.PermissionChecker
	engine      policy.Engine
}

func NewAddProjectMemberHandler(
	projectRepo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
	engine policy.Engine,
) *AddProjectMemberHandler {
	return &AddProjectMemberHandler{
		projectRepo: projectRepo, memberRepo: memberRepo, wsChecker: wsChecker,
		governance: governance, engine: engine,
	}
}

func (h *AddProjectMemberHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, actorID, targetID kernel.MemberID, projectID kernel.ProjectID, role string) error {
	if err := h.requireAdmin(ctx, wsID, actorID, projectID); err != nil {
		return err
	}
	role = normalizeProjectRole(role)
	pm := model.NewProjectMember(projectID, targetID, role)
	if err := h.memberRepo.Save(ctx, pm); err != nil {
		return err
	}
	syncProjectMemberPolicy(h.engine, targetID, wsID, projectID, role)
	return nil
}

type UpdateProjectMemberRoleHandler struct {
	projectRepo repository.ProjectRepository
	memberRepo  repository.ProjectMemberRepository
	wsChecker   port.WorkspaceChecker
	governance  authz.PermissionChecker
	engine      policy.Engine
}

func NewUpdateProjectMemberRoleHandler(
	projectRepo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
	engine policy.Engine,
) *UpdateProjectMemberRoleHandler {
	return &UpdateProjectMemberRoleHandler{
		projectRepo: projectRepo, memberRepo: memberRepo, wsChecker: wsChecker,
		governance: governance, engine: engine,
	}
}

func (h *UpdateProjectMemberRoleHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, actorID, targetID kernel.MemberID, projectID kernel.ProjectID, role string) error {
	if err := h.requireAdmin(ctx, wsID, actorID, projectID); err != nil {
		return err
	}
	role = normalizeProjectRole(role)
	if err := h.memberRepo.UpdateRole(ctx, projectID, targetID, role); err != nil {
		return err
	}
	syncProjectMemberPolicy(h.engine, targetID, wsID, projectID, role)
	return nil
}

type RemoveProjectMemberHandler struct {
	projectRepo repository.ProjectRepository
	memberRepo  repository.ProjectMemberRepository
	wsChecker   port.WorkspaceChecker
	governance  authz.PermissionChecker
	engine      policy.Engine
}

func NewRemoveProjectMemberHandler(
	projectRepo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
	engine policy.Engine,
) *RemoveProjectMemberHandler {
	return &RemoveProjectMemberHandler{
		projectRepo: projectRepo, memberRepo: memberRepo, wsChecker: wsChecker,
		governance: governance, engine: engine,
	}
}

func (h *RemoveProjectMemberHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, actorID, targetID kernel.MemberID, projectID kernel.ProjectID) error {
	if err := h.requireAdmin(ctx, wsID, actorID, projectID); err != nil {
		return err
	}
	if err := h.memberRepo.Remove(ctx, projectID, targetID); err != nil {
		return err
	}
	if ce, ok := h.engine.(*policy.CasbinEngine); ok {
		_ = ce.RemoveProjectMember(targetID.String(), wsID.String(), projectID.String())
	}
	return nil
}

func (h *AddProjectMemberHandler) requireAdmin(ctx context.Context, wsID kernel.WorkspaceID, actorID kernel.MemberID, projectID kernel.ProjectID) error {
	return requireProjectAdmin(ctx, h.projectRepo, h.memberRepo, h.wsChecker, h.governance, wsID, actorID, projectID)
}

func (h *UpdateProjectMemberRoleHandler) requireAdmin(ctx context.Context, wsID kernel.WorkspaceID, actorID kernel.MemberID, projectID kernel.ProjectID) error {
	return requireProjectAdmin(ctx, h.projectRepo, h.memberRepo, h.wsChecker, h.governance, wsID, actorID, projectID)
}

func (h *RemoveProjectMemberHandler) requireAdmin(ctx context.Context, wsID kernel.WorkspaceID, actorID kernel.MemberID, projectID kernel.ProjectID) error {
	return requireProjectAdmin(ctx, h.projectRepo, h.memberRepo, h.wsChecker, h.governance, wsID, actorID, projectID)
}

func requireProjectAdmin(
	ctx context.Context,
	projectRepo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
	wsID kernel.WorkspaceID,
	actorID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	project, err := projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.WorkspaceID != wsID {
		return domain.ErrProjectNotFound
	}
	ok, err := wsChecker.MemberInWorkspace(ctx, actorID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	if governance != nil {
		isGovAdmin, err := governance.HasPermission(ctx, wsID, actorID, authz.PermGovernanceAdmin)
		if err != nil {
			return err
		}
		if isGovAdmin {
			return nil
		}
	}
	has, role, err := memberRepo.HasAccess(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if has && role == model.ProjectRoleLead {
		return nil
	}
	return domain.ErrProjectAccessDenied
}

func normalizeProjectRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case model.ProjectRoleLead:
		return model.ProjectRoleLead
	case model.ProjectRoleViewer:
		return model.ProjectRoleViewer
	default:
		return model.ProjectRoleMember
	}
}
