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

func (h *CreateTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, rawName, kind, language, detail, rules string) (*model.Template, error) {
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
	t := model.NewTemplate(wsID, name, kind, language, detail, rules, memberID)
	if err := h.repo.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

type CreateDockerfileHandler struct {
	repo      repository.MarketRepository
	wsChecker port.WorkspaceChecker
}

func NewCreateDockerfileHandler(repo repository.MarketRepository, wsChecker port.WorkspaceChecker) *CreateDockerfileHandler {
	return &CreateDockerfileHandler{repo: repo, wsChecker: wsChecker}
}

func (h *CreateDockerfileHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, rawName, language, description, detail, dockerfile, version string) (*model.Dockerfile, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	name, err := model.NewDockerfileName(rawName)
	if err != nil {
		return nil, err
	}
	df := model.NewDockerfile(wsID, name, language, description, detail, dockerfile, version, memberID)
	if err := h.repo.SaveDockerfile(ctx, df); err != nil {
		return nil, err
	}
	return df, nil
}
