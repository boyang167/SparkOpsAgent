package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateProjectHandler struct {
	repo           repository.ProjectRepository
	memberRepo     repository.ProjectMemberRepository
	wsChecker      port.WorkspaceChecker
	clusterChecker port.WorkspaceClusterChecker
	productChecker port.ProductChecker
	engine         policy.Engine
	bus            eventbus.Bus
	tx             postgres.Transactor
}

func NewCreateProjectHandler(
	repo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	clusterChecker port.WorkspaceClusterChecker,
	productChecker port.ProductChecker,
	engine policy.Engine,
	bus eventbus.Bus,
	tx postgres.Transactor,
) *CreateProjectHandler {
	return &CreateProjectHandler{
		repo: repo, memberRepo: memberRepo, wsChecker: wsChecker,
		clusterChecker: clusterChecker, productChecker: productChecker, engine: engine, bus: bus, tx: tx,
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
	if err := validateProductMembership(ctx, h.productChecker, wsID, input.ProductID); err != nil {
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
	pm := model.NewProjectMember(project.ID, memberID, model.ProjectRoleLead)
	domainEvents := project.PullEvents()
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, project); err != nil {
			return err
		}
		if err := h.memberRepo.Save(txCtx, pm); err != nil {
			return err
		}
		return h.bus.Publish(txCtx, domainEvents...)
	})
	if err != nil {
		return nil, err
	}
	syncProjectMemberPolicy(h.engine, memberID, wsID, project.ID, model.ProjectRoleLead)
	return project, nil
}

func syncProjectMemberPolicy(engine policy.Engine, memberID kernel.MemberID, wsID kernel.WorkspaceID, projectID kernel.ProjectID, role string) {
	if ce, ok := engine.(*policy.CasbinEngine); ok {
		_ = ce.SyncProjectMemberRole(memberID.String(), wsID.String(), projectID.String(), role)
	}
}
