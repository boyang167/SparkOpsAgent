package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateProjectHandler struct {
	repo           repository.ProjectRepository
	memberRepo     repository.ProjectMemberRepository
	wsChecker      port.WorkspaceChecker
	clusterChecker port.WorkspaceClusterChecker
	engine         policy.Engine
	bus            eventbus.Bus
}

func NewCreateProjectHandler(
	repo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	clusterChecker port.WorkspaceClusterChecker,
	engine policy.Engine,
	bus eventbus.Bus,
) *CreateProjectHandler {
	return &CreateProjectHandler{
		repo: repo, memberRepo: memberRepo, wsChecker: wsChecker,
		clusterChecker: clusterChecker, engine: engine, bus: bus,
	}
}

func (h *CreateProjectHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, input model.CreateProjectInput) (*model.Project, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}

	name, err := model.NewProjectName(input.Name)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindByWorkspaceAndName(ctx, wsID, name); existing != nil {
		return nil, domain.ErrProjectNameExists
	}

	count, err := h.repo.CountByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if err := h.wsChecker.CanCreateProject(ctx, wsID, count); err != nil {
		return nil, err
	}
	if input.DefaultClusterID != "" {
		clusterID, err := kernel.ParseClusterInfoID(input.DefaultClusterID)
		if err != nil {
			return nil, domain.ErrClusterNotAuthorized
		}
		granted, err := h.clusterChecker.WorkspaceClusterGranted(ctx, wsID, clusterID)
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, domain.ErrClusterNotAuthorized
		}
	}

	project := model.NewProject(wsID, name, memberID, input)
	if err := h.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	pm := model.NewProjectMember(project.ID, memberID, model.ProjectRoleLead)
	if err := h.memberRepo.Save(ctx, pm); err != nil {
		return nil, err
	}
	syncProjectMemberPolicy(h.engine, memberID, wsID, project.ID, model.ProjectRoleLead)
	for _, e := range project.PullEvents() {
		_ = h.bus.Publish(ctx, e)
	}
	return project, nil
}

func syncProjectMemberPolicy(engine policy.Engine, memberID kernel.MemberID, wsID kernel.WorkspaceID, projectID kernel.ProjectID, role string) {
	if ce, ok := engine.(*policy.CasbinEngine); ok {
		_ = ce.SyncProjectMemberRole(memberID.String(), wsID.String(), projectID.String(), role)
	}
}
