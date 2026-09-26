package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetOrSeedSprintProcessHandler struct {
	products port.ProductChecker
	processes repository.ProcessRepository
}

func NewGetOrSeedSprintProcessHandler(products port.ProductChecker, processes repository.ProcessRepository) *GetOrSeedSprintProcessHandler {
	return &GetOrSeedSprintProcessHandler{products: products, processes: processes}
}

func (h *GetOrSeedSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) (*model.SprintProcess, error) {
	if err := h.products.ProductExists(ctx, productID); err != nil {
		return nil, err
	}
	process, err := h.processes.FindDefaultByProduct(ctx, productID)
	if err == nil {
		return process, nil
	}
	if !errors.Is(err, domain.ErrSprintProcessNotFound) {
		return nil, err
	}
	process = model.NewSprintProcess(productID, "标准敏捷流程", model.DefaultSprintStages(), actor)
	if err := h.processes.Save(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}

type UpsertSprintProcessInput struct {
	Name   string
	Stages []model.SprintStage
}

type UpsertSprintProcessHandler struct {
	processes repository.ProcessRepository
	seed      *GetOrSeedSprintProcessHandler
}

func NewUpsertSprintProcessHandler(processes repository.ProcessRepository, seed *GetOrSeedSprintProcessHandler) *UpsertSprintProcessHandler {
	return &UpsertSprintProcessHandler{processes: processes, seed: seed}
}

func (h *UpsertSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input UpsertSprintProcessInput) (*model.SprintProcess, error) {
	process, err := h.seed.Handle(ctx, productID, actor)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) != "" {
		process.Name = strings.TrimSpace(input.Name)
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
	if err := h.processes.Update(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}

type ListSprintProcessesHandler struct {
	products  port.ProductChecker
	processes repository.ProcessRepository
	seed      *GetOrSeedSprintProcessHandler
}

func NewListSprintProcessesHandler(products port.ProductChecker, processes repository.ProcessRepository, seed *GetOrSeedSprintProcessHandler) *ListSprintProcessesHandler {
	return &ListSprintProcessesHandler{products: products, processes: processes, seed: seed}
}

func (h *ListSprintProcessesHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) ([]*model.SprintProcess, error) {
	if err := h.products.ProductExists(ctx, productID); err != nil {
		return nil, err
	}
	if _, err := h.seed.Handle(ctx, productID, actor); err != nil {
		return nil, err
	}
	return h.processes.ListByProduct(ctx, productID)
}

type CreateSprintProcessInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
	IsDefault   bool
}

type CreateSprintProcessHandler struct {
	products  port.ProductChecker
	processes repository.ProcessRepository
}

func NewCreateSprintProcessHandler(products port.ProductChecker, processes repository.ProcessRepository) *CreateSprintProcessHandler {
	return &CreateSprintProcessHandler{products: products, processes: processes}
}

func (h *CreateSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input CreateSprintProcessInput) (*model.SprintProcess, error) {
	if err := h.products.ProductExists(ctx, productID); err != nil {
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
		if defaultProcess, err := h.processes.FindDefaultByProduct(ctx, productID); err == nil && defaultProcess != nil {
			defaultProcess.IsDefault = false
			if err := h.processes.Update(ctx, defaultProcess); err != nil {
				return nil, err
			}
		}
		process.IsDefault = true
	}
	if err := h.processes.Save(ctx, process); err != nil {
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
	products  port.ProductChecker
	processes repository.ProcessRepository
}

func NewUpdateSprintProcessByIDHandler(products port.ProductChecker, processes repository.ProcessRepository) *UpdateSprintProcessByIDHandler {
	return &UpdateSprintProcessByIDHandler{products: products, processes: processes}
}

func (h *UpdateSprintProcessByIDHandler) Handle(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID, input UpdateSprintProcessByIDInput) (*model.SprintProcess, error) {
	if err := h.products.ProductExists(ctx, productID); err != nil {
		return nil, err
	}
	process, err := h.processes.FindByID(ctx, productID, processID)
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
	if err := h.processes.Update(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}

type DeleteSprintProcessHandler struct {
	products  port.ProductChecker
	processes repository.ProcessRepository
	usage     port.SprintUsageChecker
}

func NewDeleteSprintProcessHandler(products port.ProductChecker, processes repository.ProcessRepository, usage port.SprintUsageChecker) *DeleteSprintProcessHandler {
	return &DeleteSprintProcessHandler{products: products, processes: processes, usage: usage}
}

func (h *DeleteSprintProcessHandler) Handle(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) error {
	if err := h.products.ProductExists(ctx, productID); err != nil {
		return err
	}
	process, err := h.processes.FindByID(ctx, productID, processID)
	if err != nil {
		return err
	}
	if process.IsDefault {
		return domain.ErrSprintProcessDefaultProtected
	}
	count, err := h.usage.CountSprintsByProcess(ctx, processID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrSprintProcessInUse
	}
	return h.processes.Delete(ctx, productID, processID)
}

type ListWorkspaceSprintTemplatesHandler struct {
	templates repository.WorkspaceTemplateRepository
}

func NewListWorkspaceSprintTemplatesHandler(templates repository.WorkspaceTemplateRepository) *ListWorkspaceSprintTemplatesHandler {
	return &ListWorkspaceSprintTemplatesHandler{templates: templates}
}

func (h *ListWorkspaceSprintTemplatesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, actor kernel.MemberID) ([]*model.WorkspaceSprintProcessTemplate, error) {
	items, err := h.templates.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	existingKeys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if key := strings.TrimSpace(item.TemplateKey); key != "" {
			existingKeys[key] = struct{}{}
		}
	}
	stageDefs := model.DefaultSprintProcessTemplates()
	seeded := false
	for _, spec := range model.BuiltinWorkspaceTemplateSpecs() {
		if _, ok := existingKeys[spec.TemplateKey]; ok {
			continue
		}
		stages, ok := stageDefs[spec.TemplateKey]
		if !ok {
			continue
		}
		tpl, err := model.NewWorkspaceSprintProcessTemplate(
			workspaceID, spec.Name, spec.TemplateKey, spec.Description, stages, actor,
		)
		if err != nil {
			return nil, err
		}
		if err := h.templates.Save(ctx, tpl); err != nil {
			return nil, err
		}
		seeded = true
	}
	if seeded {
		return h.templates.List(ctx, workspaceID)
	}
	return items, nil
}

type CreateWorkspaceSprintTemplateInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
}

type CreateWorkspaceSprintTemplateHandler struct {
	templates repository.WorkspaceTemplateRepository
}

func NewCreateWorkspaceSprintTemplateHandler(templates repository.WorkspaceTemplateRepository) *CreateWorkspaceSprintTemplateHandler {
	return &CreateWorkspaceSprintTemplateHandler{templates: templates}
}

func (h *CreateWorkspaceSprintTemplateHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, actor kernel.MemberID, input CreateWorkspaceSprintTemplateInput) (*model.WorkspaceSprintProcessTemplate, error) {
	tpl, err := model.NewWorkspaceSprintProcessTemplate(workspaceID, input.Name, input.TemplateKey, input.Description, input.Stages, actor)
	if err != nil {
		return nil, err
	}
	if err := h.templates.Save(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

type UpdateWorkspaceSprintTemplateInput struct {
	Name        string
	TemplateKey string
	Description string
	Stages      []model.SprintStage
}

type UpdateWorkspaceSprintTemplateHandler struct {
	templates repository.WorkspaceTemplateRepository
}

func NewUpdateWorkspaceSprintTemplateHandler(templates repository.WorkspaceTemplateRepository) *UpdateWorkspaceSprintTemplateHandler {
	return &UpdateWorkspaceSprintTemplateHandler{templates: templates}
}

func (h *UpdateWorkspaceSprintTemplateHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, templateID kernel.WorkspaceSprintProcessTemplateID, input UpdateWorkspaceSprintTemplateInput) (*model.WorkspaceSprintProcessTemplate, error) {
	tpl, err := h.templates.FindByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) != "" {
		tpl.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.TemplateKey) != "" {
		tpl.TemplateKey = strings.TrimSpace(input.TemplateKey)
	}
	if input.Description != "" {
		tpl.Description = strings.TrimSpace(input.Description)
	}
	if len(input.Stages) > 0 {
		stages, err := model.NormalizeSprintStages(input.Stages)
		if err != nil {
			return nil, err
		}
		tpl.Stages = stages
	}
	tpl.UpdatedAt = time.Now().UTC()
	if err := h.templates.Update(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

type DeleteWorkspaceSprintTemplateHandler struct {
	templates repository.WorkspaceTemplateRepository
}

func NewDeleteWorkspaceSprintTemplateHandler(templates repository.WorkspaceTemplateRepository) *DeleteWorkspaceSprintTemplateHandler {
	return &DeleteWorkspaceSprintTemplateHandler{templates: templates}
}

func (h *DeleteWorkspaceSprintTemplateHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, templateID kernel.WorkspaceSprintProcessTemplateID) error {
	return h.templates.Delete(ctx, workspaceID, templateID)
}

type ImportWorkspaceTemplateInput struct {
	WorkspaceTemplateID kernel.WorkspaceSprintProcessTemplateID
	Name                string
}

type ImportWorkspaceTemplateToProductHandler struct {
	products  port.ProductChecker
	processes repository.ProcessRepository
	templates repository.WorkspaceTemplateRepository
}

func NewImportWorkspaceTemplateToProductHandler(
	products port.ProductChecker,
	processes repository.ProcessRepository,
	templates repository.WorkspaceTemplateRepository,
) *ImportWorkspaceTemplateToProductHandler {
	return &ImportWorkspaceTemplateToProductHandler{products: products, processes: processes, templates: templates}
}

func (h *ImportWorkspaceTemplateToProductHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, productID kernel.ProductID, actor kernel.MemberID, input ImportWorkspaceTemplateInput) (*model.SprintProcess, error) {
	if err := h.products.ProductExists(ctx, productID); err != nil {
		return nil, err
	}
	tpl, err := h.templates.FindByID(ctx, workspaceID, input.WorkspaceTemplateID)
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
	if err := h.processes.Save(ctx, process); err != nil {
		return nil, err
	}
	return process, nil
}
