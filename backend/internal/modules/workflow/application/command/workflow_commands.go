package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateWorkflowHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewCreateWorkflowHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *CreateWorkflowHandler {
	return &CreateWorkflowHandler{repo: repo, projectChecker: projectChecker}
}

func (h *CreateWorkflowHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	rawName, description, definition string,
) (*model.Workflow, error) {
	active, err := h.projectChecker.ProjectIsActive(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, domain.ErrProjectNotActive
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}

	name, err := model.NewWorkflowName(rawName)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindByProjectAndName(ctx, projectID, name); existing != nil {
		return nil, domain.ErrWorkflowNameExists
	}

	workflow := model.NewWorkflow(projectID, name, description, definition)
	if err := h.repo.Save(ctx, workflow); err != nil {
		return nil, err
	}
	return workflow, nil
}

type UpdateWorkflowHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewUpdateWorkflowHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *UpdateWorkflowHandler {
	return &UpdateWorkflowHandler{repo: repo, projectChecker: projectChecker}
}

func (h *UpdateWorkflowHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	workflowID kernel.WorkflowID,
	rawName, description, definition string,
) (*model.Workflow, error) {
	workflow, err := h.repo.FindByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if workflow.Status == model.WorkflowStatusArchived {
		return nil, domain.ErrWorkflowNotFound
	}

	name, err := model.NewWorkflowName(rawName)
	if err != nil {
		return nil, err
	}
	if name.String() != workflow.Name.String() {
		if existing, _ := h.repo.FindByProjectAndName(ctx, workflow.ProjectID, name); existing != nil {
			return nil, domain.ErrWorkflowNameExists
		}
	}
	if err := workflow.Update(name, description, definition); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, workflow); err != nil {
		return nil, err
	}
	return workflow, nil
}

type ActivateWorkflowHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewActivateWorkflowHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *ActivateWorkflowHandler {
	return &ActivateWorkflowHandler{repo: repo, projectChecker: projectChecker}
}

func (h *ActivateWorkflowHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, workflowID kernel.WorkflowID) (*model.Workflow, error) {
	workflow, err := h.repo.FindByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if workflow.Status == model.WorkflowStatusArchived {
		return nil, domain.ErrWorkflowNotFound
	}
	if err := workflow.Activate(); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, workflow); err != nil {
		return nil, err
	}
	return workflow, nil
}

type ArchiveWorkflowHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewArchiveWorkflowHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *ArchiveWorkflowHandler {
	return &ArchiveWorkflowHandler{repo: repo, projectChecker: projectChecker}
}

func (h *ArchiveWorkflowHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, workflowID kernel.WorkflowID) error {
	workflow, err := h.repo.FindByID(ctx, workflowID)
	if err != nil {
		return err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	if err := workflow.Archive(); err != nil {
		return err
	}
	return h.repo.Update(ctx, workflow)
}
