package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ArchiveInput struct {
	ConfirmName string `json:"confirm_name"`
	Teardown    bool   `json:"teardown"`
}

type ArchiveResult struct {
	Status          string   `json:"status"`
	PendingClusters []string `json:"pending_clusters"`
}

type ArchiveProjectHandler struct {
	repo        repository.ProjectRepository
	wsChecker   port.WorkspaceChecker
	coordinator *ArchiveCoordinator
}

func NewArchiveProjectHandler(
	repo repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
	coordinator *ArchiveCoordinator,
) *ArchiveProjectHandler {
	return &ArchiveProjectHandler{repo: repo, wsChecker: wsChecker, coordinator: coordinator}
}

func (h *ArchiveProjectHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input ArchiveInput,
) (ArchiveResult, error) {
	project, err := h.load(ctx, wsID, memberID, projectID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if strings.TrimSpace(input.ConfirmName) != project.Name.String() {
		return ArchiveResult{}, domain.ErrConfirmNameMismatch
	}
	return h.archive(ctx, project, memberID, input.Teardown)
}

// HandleLegacy serves DELETE /projects/:id, which never tears environments down.
func (h *ArchiveProjectHandler) HandleLegacy(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	project, err := h.load(ctx, wsID, memberID, projectID)
	if err != nil {
		return err
	}
	_, err = h.archive(ctx, project, memberID, false)
	return err
}

func (h *ArchiveProjectHandler) load(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
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
	switch project.Status {
	case model.ProjectStatusArchived:
		return nil, domain.ErrProjectAlreadyArchived
	case model.ProjectStatusArchiving:
		return nil, domain.ErrProjectArchiving
	}
	return project, nil
}

func (h *ArchiveProjectHandler) archive(
	ctx context.Context,
	project *model.Project,
	memberID kernel.MemberID,
	withTeardown bool,
) (ArchiveResult, error) {
	if project.ProductID != nil {
		return ArchiveResult{}, domain.ErrProjectBoundToProduct
	}
	teardown, err := h.coordinator.port()
	if err != nil {
		return ArchiveResult{}, err
	}
	active, err := teardown.HasActiveRun(ctx, project.ID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if active {
		return ArchiveResult{}, domain.ErrProjectDeploymentRunning
	}
	online, err := teardown.OnlineClusters(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if len(online) == 0 {
		if err := h.coordinator.finish(ctx, teardown, project); err != nil {
			return ArchiveResult{}, err
		}
		return ArchiveResult{Status: string(model.ProjectStatusArchived), PendingClusters: []string{}}, nil
	}
	if !withTeardown {
		return ArchiveResult{}, domain.ErrProjectHasOnlineEnvironments
	}
	if err := project.BeginArchiving(); err != nil {
		return ArchiveResult{}, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return ArchiveResult{}, err
	}
	if err := h.coordinator.Advance(ctx, project, memberID); err != nil {
		if abortErr := project.AbortArchiving(); abortErr == nil {
			_ = h.repo.Update(ctx, project)
		}
		return ArchiveResult{}, err
	}
	pending := make([]string, 0, len(online))
	for _, id := range online {
		pending = append(pending, id.String())
	}
	return ArchiveResult{Status: string(model.ProjectStatusArchiving), PendingClusters: pending}, nil
}
