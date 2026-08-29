package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateProjectHandler struct {
	repo           repository.ProjectRepository
	wsChecker      port.WorkspaceChecker
	clusterChecker port.WorkspaceClusterChecker
}

func NewUpdateProjectHandler(repo repository.ProjectRepository, wsChecker port.WorkspaceChecker, clusterChecker port.WorkspaceClusterChecker) *UpdateProjectHandler {
	return &UpdateProjectHandler{repo: repo, wsChecker: wsChecker, clusterChecker: clusterChecker}
}

func (h *UpdateProjectHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input model.UpdateProjectInput,
) (*model.Project, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}

	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	if input.DefaultClusterID != nil && *input.DefaultClusterID != "" {
		clusterID, err := kernel.ParseClusterInfoID(*input.DefaultClusterID)
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

	if input.Name != nil {
		name, err := model.NewProjectName(*input.Name)
		if err != nil {
			return nil, err
		}
		if name.String() != project.Name.String() {
			if existing, _ := h.repo.FindByWorkspaceAndName(ctx, wsID, name); existing != nil {
				return nil, domain.ErrProjectNameExists
			}
		}
	}
	if err := project.ApplyPatch(input); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}
