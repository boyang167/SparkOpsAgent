package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateSprintProcessInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
	IsDefault   bool
}

type ListSprintProcessesHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	seed     *GetOrSeedSprintProcessHandler
}

func NewListSprintProcessesHandler(products repository.ProductRepository, sprints repository.SprintRepository, seed *GetOrSeedSprintProcessHandler) *ListSprintProcessesHandler {
	return &ListSprintProcessesHandler{products: products, sprints: sprints, seed: seed}
}

func (h *ListSprintProcessesHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) ([]*model.SprintProcess, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	if _, err := h.seed.Handle(ctx, productID, actor); err != nil {
		return nil, err
	}
	return h.sprints.ListProcessesByProduct(ctx, productID)
}

type CreateSprintProcessHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewCreateSprintProcessHandler(products repository.ProductRepository, sprints repository.SprintRepository) *CreateSprintProcessHandler {
	return &CreateSprintProcessHandler{products: products, sprints: sprints}
}

func (h *CreateSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input CreateSprintProcessInput) (*model.SprintProcess, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	stages, err := model.NormalizeSprintStages(input.Stages)
	if err != nil {
		return nil, err
	}
	process := model.NewSprintProcess(productID, strings.TrimSpace(input.Name), stages, actor)
	process.TemplateKey = strings.TrimSpace(input.TemplateKey)
	process.Description = strings.TrimSpace(input.Description)
	process.IsDefault = false
	if input.IsDefault {
		if defaultProcess, err := h.sprints.FindDefaultProcessByProduct(ctx, productID); err == nil && defaultProcess != nil {
			defaultProcess.IsDefault = false
			if err := h.sprints.UpdateProcess(ctx, defaultProcess); err != nil {
				return nil, err
			}
		}
		process.IsDefault = true
	}
	if err := h.sprints.SaveProcess(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}

type UpdateSprintProcessByIDInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
}

type UpdateSprintProcessByIDHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewUpdateSprintProcessByIDHandler(products repository.ProductRepository, sprints repository.SprintRepository) *UpdateSprintProcessByIDHandler {
	return &UpdateSprintProcessByIDHandler{products: products, sprints: sprints}
}

func (h *UpdateSprintProcessByIDHandler) Handle(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID, input UpdateSprintProcessByIDInput) (*model.SprintProcess, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	process, err := h.sprints.FindProcessByID(ctx, productID, processID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) != "" {
		process.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.TemplateKey) != "" {
		process.TemplateKey = strings.TrimSpace(input.TemplateKey)
	}
	if input.Description != "" {
		process.Description = strings.TrimSpace(input.Description)
	}
	if len(input.Stages) > 0 {
		stages, err := model.NormalizeSprintStages(input.Stages)
		if err != nil {
			return nil, err
		}
		process.Stages = stages
	}
	process.Version++
	process.UpdatedAt = time.Now().UTC()
	if err := h.sprints.UpdateProcess(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}

type DeleteSprintProcessHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewDeleteSprintProcessHandler(products repository.ProductRepository, sprints repository.SprintRepository) *DeleteSprintProcessHandler {
	return &DeleteSprintProcessHandler{products: products, sprints: sprints}
}

func (h *DeleteSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) error {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return err
	}
	process, err := h.sprints.FindProcessByID(ctx, productID, processID)
	if err != nil {
		return err
	}
	if process.IsDefault {
		return domain.ErrSprintProcessDefaultProtected
	}
	count, err := h.sprints.CountSprintsByProcess(ctx, processID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrSprintProcessInUse
	}
	return h.sprints.DeleteProcess(ctx, productID, processID)
}

type ListWorkspaceSprintTemplatesHandler struct {
	sprints repository.SprintRepository
}

func NewListWorkspaceSprintTemplatesHandler(sprints repository.SprintRepository) *ListWorkspaceSprintTemplatesHandler {
	return &ListWorkspaceSprintTemplatesHandler{sprints: sprints}
}

func (h *ListWorkspaceSprintTemplatesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceSprintProcessTemplate, error) {
	return h.sprints.ListWorkspaceSprintTemplates(ctx, workspaceID)
}

type CreateWorkspaceSprintTemplateInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
}

type CreateWorkspaceSprintTemplateHandler struct {
	sprints repository.SprintRepository
}

func NewCreateWorkspaceSprintTemplateHandler(sprints repository.SprintRepository) *CreateWorkspaceSprintTemplateHandler {
	return &CreateWorkspaceSprintTemplateHandler{sprints: sprints}
}

func (h *CreateWorkspaceSprintTemplateHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, actor kernel.MemberID, input CreateWorkspaceSprintTemplateInput) (*model.WorkspaceSprintProcessTemplate, error) {
	tpl, err := model.NewWorkspaceSprintProcessTemplate(workspaceID, input.Name, input.TemplateKey, input.Description, input.Stages, actor)
	if err != nil {
		return nil, err
	}
	if err := h.sprints.SaveWorkspaceSprintTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

type DeleteWorkspaceSprintTemplateHandler struct {
	sprints repository.SprintRepository
}

func NewDeleteWorkspaceSprintTemplateHandler(sprints repository.SprintRepository) *DeleteWorkspaceSprintTemplateHandler {
	return &DeleteWorkspaceSprintTemplateHandler{sprints: sprints}
}

func (h *DeleteWorkspaceSprintTemplateHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, templateID kernel.WorkspaceSprintProcessTemplateID) error {
	return h.sprints.DeleteWorkspaceSprintTemplate(ctx, workspaceID, templateID)
}

type ImportWorkspaceTemplateInput struct {
	WorkspaceTemplateID kernel.WorkspaceSprintProcessTemplateID
	Name                string
}

type ImportWorkspaceTemplateToProductHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewImportWorkspaceTemplateToProductHandler(products repository.ProductRepository, sprints repository.SprintRepository) *ImportWorkspaceTemplateToProductHandler {
	return &ImportWorkspaceTemplateToProductHandler{products: products, sprints: sprints}
}

func (h *ImportWorkspaceTemplateToProductHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, productID kernel.ProductID, actor kernel.MemberID, input ImportWorkspaceTemplateInput) (*model.SprintProcess, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	tpl, err := h.sprints.FindWorkspaceSprintTemplateByID(ctx, workspaceID, input.WorkspaceTemplateID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = tpl.Name
	}
	process := model.NewSprintProcess(productID, name, tpl.Stages, actor)
	process.TemplateKey = tpl.TemplateKey
	process.Description = tpl.Description
	process.IsDefault = false
	if err := h.sprints.SaveProcess(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}
