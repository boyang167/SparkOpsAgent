package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ArchiveProjectHandler struct {
	repo      repository.ProjectRepository
	wsChecker port.WorkspaceChecker
}

func NewArchiveProjectHandler(repo repository.ProjectRepository, wsChecker port.WorkspaceChecker) *ArchiveProjectHandler {
	return &ArchiveProjectHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ArchiveProjectHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}

	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.WorkspaceID != wsID {
		return domain.ErrProjectNotFound
	}
	if err := project.Archive(); err != nil {
		return err
	}
	return h.repo.Update(ctx, project)
}
