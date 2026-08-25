package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateTemplateHandler struct {
	repo      repository.TemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewCreateTemplateHandler(repo repository.TemplateRepository, wsChecker port.WorkspaceChecker) *CreateTemplateHandler {
	return &CreateTemplateHandler{repo: repo, wsChecker: wsChecker}
}

func (h *CreateTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	rawName, kind, language, detail, rules string,
	status model.TemplateStatus,
	isPublic bool,
) (*model.Template, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	name, err := model.NewTemplateName(rawName)
	if err != nil {
		return nil, err
	}
	t := model.NewTemplate(wsID, name, kind, language, detail, rules, status, isPublic, memberID)
	if err := h.repo.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

type UpdateTemplateHandler struct {
	repo      repository.TemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewUpdateTemplateHandler(repo repository.TemplateRepository, wsChecker port.WorkspaceChecker) *UpdateTemplateHandler {
	return &UpdateTemplateHandler{repo: repo, wsChecker: wsChecker}
}

func (h *UpdateTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	templateID kernel.TemplateID,
	rawName, kind, language, detail, rules string,
	status model.TemplateStatus,
	isPublic bool,
) (*model.Template, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	t, err := h.repo.FindByID(ctx, wsID, templateID)
	if err != nil {
		return nil, err
	}
	name, err := model.NewTemplateName(rawName)
	if err != nil {
		return nil, err
	}
	t.Update(name, kind, language, detail, rules, status, isPublic)
	if err := h.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

type DeleteTemplateHandler struct {
	repo      repository.TemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewDeleteTemplateHandler(repo repository.TemplateRepository, wsChecker port.WorkspaceChecker) *DeleteTemplateHandler {
	return &DeleteTemplateHandler{repo: repo, wsChecker: wsChecker}
}

func (h *DeleteTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	templateID kernel.TemplateID,
) error {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	if _, err := h.repo.FindByID(ctx, wsID, templateID); err != nil {
		return err
	}
	return h.repo.Delete(ctx, templateID)
}
