package command

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
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
	tx          postgres.Transactor
}

func NewArchiveProjectHandler(
	repo repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
	coordinator *ArchiveCoordinator,
	transactors ...postgres.Transactor,
) *ArchiveProjectHandler {
	handler := &ArchiveProjectHandler{repo: repo, wsChecker: wsChecker, coordinator: coordinator}
	if len(transactors) > 0 {
		handler.tx = transactors[0]
	}
	return handler
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
	return h.archiveInTransaction(ctx, project, memberID, input.Teardown)
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
	_, err = h.archiveInTransaction(ctx, project, memberID, false)
	return err
}

func (h *ArchiveProjectHandler) archiveInTransaction(
	ctx context.Context,
	project *model.Project,
	memberID kernel.MemberID,
	withTeardown bool,
) (ArchiveResult, error) {
	if h.coordinator.operations == nil {
		return h.archive(ctx, project, memberID, withTeardown)
	}
	if h.tx == nil {
		return ArchiveResult{}, errors.New("archive operation requires transaction")
	}
	eventID := uuid.NewString()
	var result ArchiveResult
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		txCtx = events.WithMetadata(txCtx, events.Metadata{
			EventID:    eventID,
			Subscriber: "workspace-app.archive-project",
		})
		var err error
		result, err = h.archive(txCtx, project, memberID, withTeardown)
		return err
	})
	return result, err
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
	if h.coordinator.operations != nil {
		if err := h.beginArchive(ctx, project, withTeardown); err != nil {
			return ArchiveResult{}, err
		}
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return ArchiveResult{}, errors.New("archive operation requires event metadata")
		}
		if err := h.coordinator.operations.Schedule(ctx, messaging.OperationRequest{
			Owner:          metadata.Subscriber,
			OperationType:  workspaceoutbound.OperationTypeProjectUndeploy,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + workspaceoutbound.OperationTypeProjectUndeploy + ":" + project.ID.String() + "/" + project.ArchiveGeneration,
			Payload: workspaceoutbound.ProjectUndeployPayload{
				EventID: metadata.EventID, ProjectID: project.ID.String(),
				MemberID: memberID.String(), AllowUndeploy: project.ArchiveAllowUndeploy, Generation: project.ArchiveGeneration,
			},
		}); err != nil {
			return ArchiveResult{}, err
		}
		return ArchiveResult{Status: string(model.ProjectStatusArchiving), PendingClusters: []string{}}, nil
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
		if err := h.beginArchive(ctx, project, withTeardown); err != nil {
			return ArchiveResult{}, err
		}
		if err := h.coordinator.finish(ctx, teardown, project); err != nil {
			return ArchiveResult{}, err
		}
		return ArchiveResult{Status: string(model.ProjectStatusArchived), PendingClusters: []string{}}, nil
	}
	if !withTeardown {
		return ArchiveResult{}, domain.ErrProjectHasOnlineEnvironments
	}
	if err := h.beginArchive(ctx, project, withTeardown); err != nil {
		return ArchiveResult{}, err
	}
	if err := h.coordinator.Advance(ctx, project, memberID); err != nil {
		return ArchiveResult{}, err
	}
	pending := make([]string, 0, len(online))
	for _, id := range online {
		pending = append(pending, id.String())
	}
	return ArchiveResult{Status: string(model.ProjectStatusArchiving), PendingClusters: pending}, nil
}

func (h *ArchiveProjectHandler) beginArchive(
	ctx context.Context,
	project *model.Project,
	allowUndeploy bool,
) error {
	terminal, ok := h.repo.(repository.ProjectTerminalRepository)
	if !ok {
		return errors.New("archive requires lifecycle transition repository")
	}
	generation := uuid.NewString()
	begun, err := terminal.BeginArchive(ctx, project.ID, generation, allowUndeploy)
	if err != nil {
		return err
	}
	if !begun {
		return domain.ErrProjectNotActive
	}
	project.Status = model.ProjectStatusArchiving
	project.ArchiveGeneration = generation
	project.ArchivePhase = model.ArchivePhasePending
	project.ArchiveAllowUndeploy = boolPointer(allowUndeploy)
	return nil
}
